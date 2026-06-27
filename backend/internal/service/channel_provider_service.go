// Package service 提供业务逻辑和领域服务。
package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
)

// ChannelProvider 表示一个上游渠道商的领域模型。
//
// 渠道商按 base_url 维度聚合：一个 baseUrl 可能对应多条账号，但充值金额、
// 余额等是渠道商维度的。该结构体仅保存可手动编辑的充值金额与定期刷新得到的
// 余额快照，账号数据仍以 accounts 表为准。
type ChannelProvider struct {
	ID               int64
	BaseURL          string
	DisplayName      *string
	RechargeAmount   float64
	QuotaPerUnit     int64 // NewAPI 类 quota→USD 系数（默认 500000），仅 /api/user/self 查询用
	Balance          *float64
	BalanceUnit      string
	BalanceCheckedAt *time.Time
	IsValid          bool
	SyncBalance      bool // 是否参与"刷新全部"；关闭后刷新全部时跳过，单行刷新不受影响
	LastRefreshError *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ChannelProviderAggregated 是列表视图：渠道商信息 + 该 baseUrl 下的账号数量。
// 该 DTO 用于 handler 响应，不含任何 apiKey。
type ChannelProviderAggregated struct {
	ChannelProvider
	AccountCount int64 `json:"account_count"`
}

// ProviderRefreshSource 仅在 service 内部使用：刷新余额时，从 accounts 表中
// 取出该 baseUrl 下第一个有效 api_key 账号所需的上游调用凭据。
// WARNING: 含 apiKey，绝不能透出到 handler 响应。
type ProviderRefreshSource struct {
	APIKey  string
	BaseURL string
	Proxy   *Proxy
}

// RefreshResult 是 RefreshAllBalances 的单行结果。
type RefreshResult struct {
	BaseURL string `json:"base_url"`
	Success bool   `json:"success"`
	Skipped bool   `json:"skipped,omitempty"` // sync_balance=false，刷新全部时被跳过
	Message string `json:"message,omitempty"`
}

// ChannelProviderRepository 定义渠道商数据访问接口。
type ChannelProviderRepository interface {
	// ListAll 返回所有渠道商（不含聚合的账号数量）。
	ListAll(ctx context.Context) ([]*ChannelProvider, error)
	// GetByBaseURL 按标准化后的 baseURL 查询单个渠道商。
	GetByBaseURL(ctx context.Context, baseURL string) (*ChannelProvider, error)
	// UpdateProvider 按 base_url 插入或更新可编辑字段（充值金额 / 名称 / quota 系数）。
	UpdateProvider(ctx context.Context, baseURL string, rechargeAmount float64, displayName string, quotaPerUnit int64) error
	// SetSyncBalance 切换是否参与"刷新全部"的余额同步。
	SetSyncBalance(ctx context.Context, baseURL string, enabled bool) error
	// UpdateBalance 更新余额相关字段。
	UpdateBalance(ctx context.Context, baseURL string, balance *float64, unit string, isValid bool, errMsg string, checkedAt time.Time) error
	// ListAggregated 聚合 accounts 表按 base_url 去重，LEFT JOIN channel_providers，
	// 返回每行 base_url + account_count + 充值/余额信息。
	ListAggregated(ctx context.Context) ([]ChannelProviderAggregated, error)
	// FindAllActiveAPIKeyAccountsByBaseURL 取该标准化 baseUrl 下所有
	// status='active' 且 credentials 含可用 api_key 的账号的上游调用凭据。
	// NewAPI 类余额刷新需遍历所有账号累加 usage，因此返回全部而非单个。
	FindAllActiveAPIKeyAccountsByBaseURL(ctx context.Context, normalizedBaseURL string) ([]*ProviderRefreshSource, error)
}

// ChannelProviderService 提供渠道商列表、充值金额编辑、余额刷新等业务能力。
type ChannelProviderService struct {
	providerRepo ChannelProviderRepository
	accountRepo  AccountRepository
}

// NewChannelProviderService 构造渠道商服务。
func NewChannelProviderService(providerRepo ChannelProviderRepository, accountRepo AccountRepository) *ChannelProviderService {
	return &ChannelProviderService{providerRepo: providerRepo, accountRepo: accountRepo}
}

// channelProviderRefreshConcurrency 限制 RefreshAllBalances 的并发刷新数。
const channelProviderRefreshConcurrency = 5

// channelProviderRefreshTimeout 是单次上游 /v1/usage 调用的超时。
const channelProviderRefreshTimeout = 15 * time.Second

// List 返回渠道商聚合列表（不含 apiKey）。
func (s *ChannelProviderService) List(ctx context.Context) ([]ChannelProviderAggregated, error) {
	return s.providerRepo.ListAggregated(ctx)
}

// UpdateProvider 更新某个渠道商的可编辑字段（充值金额 / 名称 / quota 系数）。baseURL 会被标准化。
func (s *ChannelProviderService) UpdateProvider(ctx context.Context, baseURL string, rechargeAmount float64, displayName string, quotaPerUnit int64) error {
	normalized := NormalizeBaseURL(baseURL)
	if normalized == "" {
		return infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_BASE_URL", "base_url is required")
	}
	if rechargeAmount < 0 {
		return infraerrors.BadRequest("CHANNEL_PROVIDER_INVALID_AMOUNT", "recharge_amount must be >= 0")
	}
	if quotaPerUnit <= 0 {
		quotaPerUnit = 500000
	}
	return s.providerRepo.UpdateProvider(ctx, normalized, rechargeAmount, displayName, quotaPerUnit)
}

// SetSyncBalance 切换是否参与"刷新全部"的余额同步。baseURL 会被标准化。
func (s *ChannelProviderService) SetSyncBalance(ctx context.Context, baseURL string, enabled bool) error {
	normalized := NormalizeBaseURL(baseURL)
	if normalized == "" {
		return infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_BASE_URL", "base_url is required")
	}
	return s.providerRepo.SetSyncBalance(ctx, normalized, enabled)
}

// RefreshBalance 刷新单个渠道商的余额：取该 baseUrl 下任一有效 api_key 账号，
// 调 GET {base}/v1/usage，按 fallback 规则提取余额并更新本地。
// 失败时把错误写入 last_refresh_error、is_valid=false。
func (s *ChannelProviderService) RefreshBalance(ctx context.Context, baseURL string) (*ChannelProvider, error) {
	normalized := NormalizeBaseURL(baseURL)
	if normalized == "" {
		return nil, infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_BASE_URL", "base_url is required")
	}

	sources, err := s.providerRepo.FindAllActiveAPIKeyAccountsByBaseURL(ctx, normalized)
	if err != nil {
		s.recordRefreshFailure(ctx, normalized, "find active accounts failed: "+err.Error())
		return nil, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_ACCOUNT",
			"no active api_key account found for base_url: %s", normalized)
	}
	if len(sources) == 0 {
		s.recordRefreshFailure(ctx, normalized, "no active api_key account found")
		return nil, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_ACCOUNT",
			"no active api_key account found for base_url: %s", normalized)
	}

	// 读取渠道商配置：recharge_amount 作为 NewAPI 类总额度（余额 = 充值额 − 各账号已用之和）
	var rechargeAmount float64
	if existing, _ := s.providerRepo.GetByBaseURL(ctx, normalized); existing != nil {
		rechargeAmount = existing.RechargeAmount
	}

	balance, unit, isValid, refreshErr := s.fetchUpstreamBalance(ctx, sources, rechargeAmount)
	checkedAt := time.Now()
	if refreshErr != nil {
		s.recordRefreshFailure(ctx, normalized, refreshErr.Error())
		// 返回错误给调用方，便于单行刷新时前端提示
		return nil, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REFRESH_FAILED",
			"refresh balance failed: %v", refreshErr)
	}

	errMsg := ""
	if err := s.providerRepo.UpdateBalance(ctx, normalized, balance, unit, isValid, errMsg, checkedAt); err != nil {
		return nil, err
	}

	return s.providerRepo.GetByBaseURL(ctx, normalized)
}

// RefreshAllBalances 并发刷新所有渠道商余额，并发上限 5。单个失败不中断其他。
// sync_balance=false 的渠道商会被跳过（标记 Skipped，不发起上游请求）；单行刷新不受此开关影响。
func (s *ChannelProviderService) RefreshAllBalances(ctx context.Context) ([]RefreshResult, error) {
	providers, err := s.providerRepo.ListAggregated(ctx)
	if err != nil {
		return nil, err
	}

	// 结果按 providers 顺序填充，便于前端汇总成功/失败/跳过。
	results := make([]RefreshResult, len(providers))
	type refreshTask struct {
		idx     int
		baseURL string
	}
	tasks := make([]refreshTask, 0, len(providers))
	for i := range providers {
		baseURL := NormalizeBaseURL(providers[i].BaseURL)
		if baseURL == "" {
			continue
		}
		if !providers[i].SyncBalance {
			results[i] = RefreshResult{BaseURL: baseURL, Success: true, Skipped: true}
			continue
		}
		tasks = append(tasks, refreshTask{idx: i, baseURL: baseURL})
	}

	sem := make(chan struct{}, channelProviderRefreshConcurrency)
	var wg sync.WaitGroup

	for _, tk := range tasks {
		idx, baseURL := tk.idx, tk.baseURL
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// 单个渠道商刷新失败不影响整体。每个子调用用独立 context，
			// 避免某个上游超时取消整个批次。
			callCtx, cancel := context.WithTimeout(ctx, channelProviderRefreshTimeout+5*time.Second)
			defer cancel()

			_, err := s.RefreshBalance(callCtx, baseURL)
			if err != nil {
				results[idx] = RefreshResult{BaseURL: baseURL, Success: false, Message: err.Error()}
				return
			}
			results[idx] = RefreshResult{BaseURL: baseURL, Success: true}
		}()
	}
	wg.Wait()

	return results, nil
}

// fetchUpstreamBalance 刷新余额，适配两类上游：
//  1. sub2api 类：GET /v1/usage（用第一个账号，直接返回账户余额）
//  2. NewAPI(OneAPI 系) 类：/v1/usage 不存在(404) → 遍历所有账号的 /v1/dashboard/billing/usage，
//     累加 total_usage，余额 = recharge_amount − Σ total_usage/100
//
// NewAPI 类用「充值额 − 各 key 实际用量之和」得到账户维度余额，
// 绕过了 sk- key 无限额度导致 billing/subscription 返回固定 1 亿的问题。
func (s *ChannelProviderService) fetchUpstreamBalance(ctx context.Context, sources []*ProviderRefreshSource, rechargeAmount float64) (*float64, string, bool, error) {
	if len(sources) == 0 {
		return nil, "USD", false, infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_SOURCE", "no refresh source")
	}

	// 第一个账号用于试 /v1/usage（sub2api 类）
	first := sources[0]
	apiKey := strings.TrimSpace(first.APIKey)
	base := strings.TrimSpace(first.BaseURL)
	if apiKey == "" || base == "" {
		return nil, "USD", false, infraerrors.BadRequest("CHANNEL_PROVIDER_INVALID_CREDENTIALS", "api_key or base_url is empty")
	}

	proxyURL := ""
	if first.Proxy != nil {
		proxyURL = first.Proxy.URL()
	}
	client, err := httpclient.GetClient(httpclient.Options{
		ProxyURL: proxyURL,
		Timeout:  channelProviderRefreshTimeout,
	})
	if err != nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_CLIENT_ERROR",
			"build http client failed: %v", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, channelProviderRefreshTimeout)
	defer cancel()

	// 1) sub2api 类：GET /v1/usage（用第一个账号）
	if balance, unit, isValid, hit, err := s.tryUsageEndpoint(callCtx, client, apiKey, base); err != nil {
		return nil, "USD", false, err
	} else if hit {
		return balance, unit, isValid, nil
	}

	// 2) NewAPI 类：遍历所有账号累加 billing/usage
	return s.tryNewAPIUsageSum(callCtx, sources, rechargeAmount)
}

// tryUsageEndpoint 请求 /v1/usage（sub2api 类上游接口）。
// hit=true 表示端点存在并已解析；hit=false 表示 404，调用方应尝试 billing 回退。
func (s *ChannelProviderService) tryUsageEndpoint(ctx context.Context, client *http.Client, apiKey, base string) (balance *float64, unit string, isValid bool, hit bool, err error) {
	endpoint := buildOpenAIEndpointURL(base, "/v1/usage")
	statusCode, body, getErr := doUpstreamGet(ctx, client, apiKey, endpoint)
	if getErr != nil {
		return nil, "USD", false, false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REQUEST_FAILED",
			"upstream /v1/usage request failed: %v", getErr)
	}
	if statusCode == http.StatusNotFound {
		return nil, "USD", false, false, nil // 端点不存在，交由调用方 fallback
	}
	if statusCode < 200 || statusCode >= 300 {
		snippet := truncateBody(string(body), 240)
		slog.Warn("channel_provider_refresh_non_2xx",
			"endpoint", "/v1/usage", "status", statusCode, "body", snippet)
		return nil, "USD", false, false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_UPSTREAM_ERROR",
			"/v1/usage returned %d: %s", statusCode, snippet)
	}

	var payload usageResponse
	if jsonErr := json.Unmarshal(body, &payload); jsonErr != nil {
		snippet := truncateBody(string(body), 240)
		return nil, "USD", false, false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_PARSE_FAILED",
			"parse /v1/usage response failed: %v, body: %s", jsonErr, snippet)
	}

	balance, unit, isValid = extractBalance(&payload)
	return balance, unit, isValid, true, nil
}

// tryNewAPIUsageSum 遍历该 baseUrl 下所有账号，各自调 /v1/dashboard/billing/usage，
// 累加 total_usage（该 key 已用，单位美分），余额 = recharge_amount − Σ total_usage/100。
//
// 这种方式得到「账户维度」余额（用户充值额 − 所有 key 实际消耗），
// 绕过了 NewAPI 对无限额度 token 在 billing/subscription 返回固定 1 亿的问题。
// 单个账号 usage 拉取失败不致命（按 0 处理），不影响其他账号累加。
func (s *ChannelProviderService) tryNewAPIUsageSum(ctx context.Context, sources []*ProviderRefreshSource, rechargeAmount float64) (*float64, string, bool, error) {
	var totalUsageCent float64
	fetched := 0
	for _, src := range sources {
		apiKey := strings.TrimSpace(src.APIKey)
		base := strings.TrimSpace(src.BaseURL)
		if apiKey == "" || base == "" {
			continue
		}
		proxyURL := ""
		if src.Proxy != nil {
			proxyURL = src.Proxy.URL()
		}
		client, err := httpclient.GetClient(httpclient.Options{
			ProxyURL: proxyURL,
			Timeout:  channelProviderRefreshTimeout,
		})
		if err != nil {
			slog.Warn("channel_provider_usage_client_failed", "base_url", base, "err", err)
			continue
		}
		endpoint := buildOpenAIEndpointURL(base, "/v1/dashboard/billing/usage")
		statusCode, body, err := doUpstreamGet(ctx, client, apiKey, endpoint)
		if err != nil || statusCode < 200 || statusCode >= 300 {
			slog.Warn("channel_provider_usage_fetch_failed",
				"base_url", base, "status", statusCode, "err", err)
			continue
		}
		var usage billingUsage
		if json.Unmarshal(body, &usage) == nil && usage.TotalUsage != nil {
			totalUsageCent += *usage.TotalUsage
			fetched++
		}
	}

	if fetched == 0 {
		return nil, "USD", false, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_USAGE_ENDPOINT",
			"upstream supports neither /v1/usage nor /v1/dashboard/billing/usage")
	}

	balance := rechargeAmount - totalUsageCent/100
	// 充值额未设置(0)时余额为负且无意义，标记无效提示用户先设置充值金额
	isValid := rechargeAmount > 0
	return &balance, "USD", isValid, nil
}

// doUpstreamGet 发起带 Bearer 鉴权的 GET，返回状态码与 body（最多 1MB，防止异常上游撑爆内存）。
func doUpstreamGet(ctx context.Context, client *http.Client, apiKey, endpoint string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

// recordRefreshFailure 记录刷新失败到 last_refresh_error / is_valid，失败仅记日志。
func (s *ChannelProviderService) recordRefreshFailure(ctx context.Context, normalizedBaseURL, message string) {
	if err := s.providerRepo.UpdateBalance(ctx, normalizedBaseURL, nil, "USD", false, message, time.Now()); err != nil {
		slog.Warn("channel_provider_record_failure_failed",
			"base_url", normalizedBaseURL, "err", err)
	}
}

// usageResponse 是 sub2api 类 /v1/usage 响应的宽松投影。
// 使用指针字段以区分"未返回"与"零值"。兼容：顶层 remaining、嵌套 quota.remaining、顶层 balance。
type usageResponse struct {
	Remaining *float64    `json:"remaining"`
	Balance   *float64    `json:"balance"`
	Unit      *string     `json:"unit"`
	IsValid   *bool       `json:"isValid"` // sub2api 返回驼峰 isValid
	Quota     *quotaBlock `json:"quota"`
}

type quotaBlock struct {
	Remaining *float64 `json:"remaining"`
	Unit      *string  `json:"unit"`
}

// billingUsage 是 NewAPI /v1/dashboard/billing/usage 响应，total_usage 单位为美分。
type billingUsage struct {
	TotalUsage *float64 `json:"total_usage"`
}

// extractBalance 从 /v1/usage 响应按 fallback 顺序提取余额、单位、有效性。
//   - remaining: resp.Remaining ?? resp.Quota.Remaining ?? resp.Balance
//   - unit:      resp.Unit ?? resp.Quota.Unit ?? "USD"
//   - isValid:   resp.IsValid ?? true
func extractBalance(resp *usageResponse) (balance *float64, unit string, isValid bool) {
	unit = "USD"
	isValid = true
	if resp == nil {
		return
	}

	switch {
	case resp.Remaining != nil:
		v := *resp.Remaining
		balance = &v
	case resp.Quota != nil && resp.Quota.Remaining != nil:
		v := *resp.Quota.Remaining
		balance = &v
	case resp.Balance != nil:
		v := *resp.Balance
		balance = &v
	}

	switch {
	case resp.Unit != nil && *resp.Unit != "":
		unit = *resp.Unit
	case resp.Quota != nil && resp.Quota.Unit != nil && *resp.Quota.Unit != "":
		unit = *resp.Quota.Unit
	}

	if resp.IsValid != nil {
		isValid = *resp.IsValid
	}

	return
}

// NormalizeBaseURL 标准化 baseUrl：去首尾空白、去尾部斜杠、转小写。
// SQL 侧的去重表达式 LOWER(TRIM(TRAILING '/' FROM ...)) 必须与此保持一致。
func NormalizeBaseURL(s string) string {
	return strings.ToLower(strings.TrimRight(strings.TrimSpace(s), "/"))
}

// truncateBody 截断超长响应体，便于记录到错误信息 / 日志。
func truncateBody(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}
