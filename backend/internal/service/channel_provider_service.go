// Package service 提供业务逻辑和领域服务。
package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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
	Balance          *float64
	BalanceUnit      string
	BalanceCheckedAt *time.Time
	IsValid          bool
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
	Message string `json:"message,omitempty"`
}

// ChannelProviderRepository 定义渠道商数据访问接口。
type ChannelProviderRepository interface {
	// ListAll 返回所有渠道商（不含聚合的账号数量）。
	ListAll(ctx context.Context) ([]*ChannelProvider, error)
	// GetByBaseURL 按标准化后的 baseURL 查询单个渠道商。
	GetByBaseURL(ctx context.Context, baseURL string) (*ChannelProvider, error)
	// Upsert 按 base_url 唯一约束插入或更新（用于充值金额编辑）。
	Upsert(ctx context.Context, p *ChannelProvider) error
	// UpdateBalance 更新余额相关字段。
	UpdateBalance(ctx context.Context, baseURL string, balance *float64, unit string, isValid bool, errMsg string, checkedAt time.Time) error
	// ListAggregated 聚合 accounts 表按 base_url 去重，LEFT JOIN channel_providers，
	// 返回每行 base_url + account_count + 充值/余额信息。
	ListAggregated(ctx context.Context) ([]ChannelProviderAggregated, error)
	// FindFirstActiveAPIKeyAccountByBaseURL 取该标准化 baseUrl 下第一个
	// status='active' AND type='api_key' 的账号的上游调用凭据。
	FindFirstActiveAPIKeyAccountByBaseURL(ctx context.Context, normalizedBaseURL string) (*ProviderRefreshSource, error)
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

// UpdateRechargeAmount 更新某个渠道商的充值金额。baseURL 会被标准化。
func (s *ChannelProviderService) UpdateRechargeAmount(ctx context.Context, baseURL string, amount float64) error {
	normalized := NormalizeBaseURL(baseURL)
	if normalized == "" {
		return infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_BASE_URL", "base_url is required")
	}
	if amount < 0 {
		return infraerrors.BadRequest("CHANNEL_PROVIDER_INVALID_AMOUNT", "recharge_amount must be >= 0")
	}
	p := &ChannelProvider{
		BaseURL:        normalized,
		RechargeAmount: amount,
	}
	if err := s.providerRepo.Upsert(ctx, p); err != nil {
		return err
	}
	return nil
}

// RefreshBalance 刷新单个渠道商的余额：取该 baseUrl 下任一有效 api_key 账号，
// 调 GET {base}/v1/usage，按 fallback 规则提取余额并更新本地。
// 失败时把错误写入 last_refresh_error、is_valid=false。
func (s *ChannelProviderService) RefreshBalance(ctx context.Context, baseURL string) (*ChannelProvider, error) {
	normalized := NormalizeBaseURL(baseURL)
	if normalized == "" {
		return nil, infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_BASE_URL", "base_url is required")
	}

	source, err := s.providerRepo.FindFirstActiveAPIKeyAccountByBaseURL(ctx, normalized)
	if err != nil {
		s.recordRefreshFailure(ctx, normalized, "find active account failed: "+err.Error())
		return nil, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_ACCOUNT",
			"no active api_key account found for base_url: %s", normalized)
	}
	if source == nil {
		s.recordRefreshFailure(ctx, normalized, "no active api_key account found")
		return nil, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_ACCOUNT",
			"no active api_key account found for base_url: %s", normalized)
	}

	balance, unit, isValid, refreshErr := s.fetchUpstreamBalance(ctx, source)
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
func (s *ChannelProviderService) RefreshAllBalances(ctx context.Context) ([]RefreshResult, error) {
	providers, err := s.providerRepo.ListAggregated(ctx)
	if err != nil {
		return nil, err
	}

	// 基于 ListAggregated 的结果派生待刷新的 base_url 列表（已天然去重）。
	baseURLs := make([]string, 0, len(providers))
	for i := range providers {
		if b := NormalizeBaseURL(providers[i].BaseURL); b != "" {
			baseURLs = append(baseURLs, b)
		}
	}

	results := make([]RefreshResult, len(baseURLs))
	sem := make(chan struct{}, channelProviderRefreshConcurrency)
	var wg sync.WaitGroup

	for i, raw := range baseURLs {
		baseURL := raw
		idx := i
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

// fetchUpstreamBalance 调用 GET {base}/v1/usage 并按 fallback 规则提取余额。
// 返回 (balance, unit, isValid, error)。balance 为 nil 表示上游未返回余额字段。
// fetchUpstreamBalance 刷新余额，自动适配两类上游：
//  1. sub2api 类：GET /v1/usage（顶层 remaining / quota.remaining）
//  2. NewAPI(OneAPI 系) 类：/v1/usage 不存在(404) → 回退到 OpenAI 兼容 billing 接口
//     GET /v1/dashboard/billing/subscription (hard_limit_usd) + /v1/dashboard/billing/usage (total_usage 美分)
//     余额 = hard_limit_usd − total_usage/100
//
// URL 拼接复用 buildOpenAIEndpointURL，自动处理 base_url 是否已含 /v1 版本后缀。
// 整个流程（含 fallback）共享一个超时，避免逐请求叠加超时。
func (s *ChannelProviderService) fetchUpstreamBalance(ctx context.Context, source *ProviderRefreshSource) (*float64, string, bool, error) {
	if source == nil {
		return nil, "USD", false, infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_SOURCE", "refresh source is nil")
	}

	apiKey := strings.TrimSpace(source.APIKey)
	base := strings.TrimSpace(source.BaseURL)
	if apiKey == "" || base == "" {
		return nil, "USD", false, infraerrors.BadRequest("CHANNEL_PROVIDER_INVALID_CREDENTIALS", "api_key or base_url is empty")
	}

	proxyURL := ""
	if source.Proxy != nil {
		proxyURL = source.Proxy.URL()
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

	// 1) sub2api 类：GET /v1/usage
	if balance, unit, isValid, hit, err := s.tryUsageEndpoint(callCtx, client, apiKey, base); err != nil {
		return nil, "USD", false, err
	} else if hit {
		return balance, unit, isValid, nil
	}

	// 2) NewAPI(OpenAI 兼容)：GET /v1/dashboard/billing/subscription + /usage
	if balance, unit, isValid, hit, err := s.tryNewAPIBilling(callCtx, client, apiKey, base); err != nil {
		return nil, "USD", false, err
	} else if hit {
		return balance, unit, isValid, nil
	}

	// 3) NewAPI(原生)：GET /api/user/self
	return s.tryNewAPIUserSelf(callCtx, client, apiKey, base)
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

// tryNewAPIBilling 请求 NewAPI(OneAPI 系) 的 OpenAI 兼容 billing 接口：
//   - GET /v1/dashboard/billing/subscription → hard_limit_usd（总额度，USD）
//   - GET /v1/dashboard/billing/usage        → total_usage（已用，单位美分）
//
// 余额 = hard_limit_usd − total_usage/100。subscription 必须成功且含 hard_limit_usd；
// usage 拉取失败不致命（按 0 已用处理）。404 时 hit=false，交由调用方继续 fallback。
func (s *ChannelProviderService) tryNewAPIBilling(ctx context.Context, client *http.Client, apiKey, base string) (*float64, string, bool, bool, error) {
	subEndpoint := buildOpenAIEndpointURL(base, "/v1/dashboard/billing/subscription")
	statusCode, body, err := doUpstreamGet(ctx, client, apiKey, subEndpoint)
	if err != nil {
		return nil, "USD", false, false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REQUEST_FAILED",
			"upstream billing/subscription request failed: %v", err)
	}
	if statusCode == http.StatusNotFound {
		return nil, "USD", false, false, nil // 端点不存在，交由调用方继续 fallback
	}
	if statusCode < 200 || statusCode >= 300 {
		snippet := truncateBody(string(body), 240)
		slog.Warn("channel_provider_refresh_non_2xx",
			"endpoint", "/v1/dashboard/billing/subscription", "status", statusCode, "body", snippet)
		return nil, "USD", false, false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_UPSTREAM_ERROR",
			"billing/subscription returned %d: %s", statusCode, snippet)
	}

	var sub billingSubscription
	if jsonErr := json.Unmarshal(body, &sub); jsonErr != nil {
		snippet := truncateBody(string(body), 240)
		return nil, "USD", false, false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_PARSE_FAILED",
			"parse billing/subscription failed: %v, body: %s", jsonErr, snippet)
	}
	if sub.HardLimitUSD == nil {
		return nil, "USD", false, false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_NO_HARD_LIMIT",
			"billing/subscription has no hard_limit_usd field: %s", truncateBody(string(body), 240))
	}

	balance := *sub.HardLimitUSD

	// 拉取已用量（美分），失败不致命，按 0 处理
	usageEndpoint := buildOpenAIEndpointURL(base, "/v1/dashboard/billing/usage")
	if uStatus, uBody, uErr := doUpstreamGet(ctx, client, apiKey, usageEndpoint); uErr == nil && uStatus >= 200 && uStatus < 300 {
		var usage billingUsage
		if json.Unmarshal(uBody, &usage) == nil && usage.TotalUsage != nil {
			balance -= *usage.TotalUsage / 100 // cent → USD
		}
	}

	return &balance, "USD", true, true, nil
}

// tryNewAPIUserSelf 请求 NewAPI 原生接口 GET /api/user/self。
// 与 /v1/* 不同，/api/* 是管理路径，需从 base_url 提取 host（去掉 /v1 path）后拼接。
// 鉴权：Bearer sk-xxx（多数 NewAPI 部署接受 API key）或用户 access token。
// 返回 data.quota（NewAPI 内部计费点，充值增加、消费减少，即当前剩余余额），
// 按默认 QuotaPerUnit=500000 换算成 USD。
func (s *ChannelProviderService) tryNewAPIUserSelf(ctx context.Context, client *http.Client, apiKey, base string) (*float64, string, bool, error) {
	endpoint := buildAPIPathURL(base, "/api/user/self")
	statusCode, body, err := doUpstreamGet(ctx, client, apiKey, endpoint)
	if err != nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REQUEST_FAILED",
			"upstream /api/user/self request failed: %v", err)
	}
	if statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden {
		// sk- key 被拒，说明该 NewAPI 部署要求用户 access token 而非 API key
		return nil, "USD", false, infraerrors.Newf(http.StatusUnauthorized, "CHANNEL_PROVIDER_NEED_ACCESS_TOKEN",
			"/api/user/self rejected api_key (HTTP %d); this NewAPI deployment likely requires a user access token instead of an sk- key", statusCode)
	}
	if statusCode == http.StatusNotFound {
		return nil, "USD", false, infraerrors.Newf(http.StatusNotFound, "CHANNEL_PROVIDER_NO_USAGE_ENDPOINT",
			"upstream supports none of /v1/usage, /v1/dashboard/billing/subscription, /api/user/self")
	}
	if statusCode < 200 || statusCode >= 300 {
		snippet := truncateBody(string(body), 240)
		slog.Warn("channel_provider_refresh_non_2xx",
			"endpoint", "/api/user/self", "status", statusCode, "body", snippet)
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_UPSTREAM_ERROR",
			"/api/user/self returned %d: %s", statusCode, snippet)
	}

	var resp userSelfResponse
	if jsonErr := json.Unmarshal(body, &resp); jsonErr != nil {
		snippet := truncateBody(string(body), 240)
		return nil, "USD", false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_PARSE_FAILED",
			"parse /api/user/self failed: %v, body: %s", jsonErr, snippet)
	}
	if resp.Data == nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_NO_QUOTA",
			"/api/user/self response has no data field: %s", truncateBody(string(body), 240))
	}

	// NewAPI quota 是内部计费点：充值增加、消费减少，data.quota 即当前剩余余额。
	// 默认 QuotaPerUnit = 500000（即 500000 点 = 1 USD）。
	const newAPIQuotaPerUnit = 500000.0
	balance := float64(resp.Data.Quota) / newAPIQuotaPerUnit
	return &balance, "USD", true, nil
}

// buildAPIPathURL 从 base_url 提取 scheme://host（去掉 /v1 等 path 部分），拼接管理路径。
// 用于 NewAPI 的 /api/* 接口：base_url 通常存到 /v1 这一级（推理路径），
// 而 /api/user/self 在 host 根下，与 /v1 平级。
func buildAPIPathURL(base, path string) string {
	trimmed := strings.TrimSpace(base)
	if parsed, perr := url.Parse(trimmed); perr == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host + "/" + strings.TrimLeft(path, "/")
	}
	// 解析失败兜底：直接拼接
	return strings.TrimRight(trimmed, "/") + "/" + strings.TrimLeft(path, "/")
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

// billingSubscription 是 NewAPI(OneAPI 系) /v1/dashboard/billing/subscription 响应。
// 只取计算余额所需的 hard_limit_usd（总额度，USD）。
type billingSubscription struct {
	HardLimitUSD *float64 `json:"hard_limit_usd"`
}

// billingUsage 是 NewAPI /v1/dashboard/billing/usage 响应，total_usage 单位为美分。
type billingUsage struct {
	TotalUsage *float64 `json:"total_usage"`
}

// userSelfResponse 是 NewAPI 原生接口 /api/user/self 响应。
// data.quota 是当前剩余余额（内部计费点，充值增加、消费减少）。
type userSelfResponse struct {
	Success bool   `json:"success"` // NewAPI 风格
	Code    int    `json:"code"`    // OneAPI 旧风格（0=成功）
	Message string `json:"message"`
	Data    *struct {
		Quota     int64 `json:"quota"`
		UsedQuota int64 `json:"used_quota"`
	} `json:"data"`
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
