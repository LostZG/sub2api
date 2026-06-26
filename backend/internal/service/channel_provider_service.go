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
func (s *ChannelProviderService) fetchUpstreamBalance(ctx context.Context, source *ProviderRefreshSource) (*float64, string, bool, error) {
	if source == nil {
		return nil, "USD", false, infraerrors.BadRequest("CHANNEL_PROVIDER_EMPTY_SOURCE", "refresh source is nil")
	}

	apiKey := strings.TrimSpace(source.APIKey)
	base := strings.TrimRight(strings.TrimSpace(source.BaseURL), "/")
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

	endpoint := base + "/v1/usage"
	callCtx, cancel := context.WithTimeout(ctx, channelProviderRefreshTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(callCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REQUEST_BUILD_FAILED",
			"build request failed: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_REQUEST_FAILED",
			"upstream request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB 上限，防止异常上游撑爆内存
	if err != nil {
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_READ_BODY_FAILED",
			"read response body failed: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := truncateBody(string(body), 240)
		slog.Warn("channel_provider_refresh_non_2xx",
			"base_url", base, "status", resp.StatusCode, "body", snippet)
		return nil, "USD", false, infraerrors.Newf(http.StatusBadGateway, "CHANNEL_PROVIDER_UPSTREAM_ERROR",
			"upstream returned %d: %s", resp.StatusCode, snippet)
	}

	var payload usageResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		snippet := truncateBody(string(body), 240)
		return nil, "USD", false, infraerrors.Newf(http.StatusInternalServerError, "CHANNEL_PROVIDER_PARSE_FAILED",
			"parse usage response failed: %v, body: %s", err, snippet)
	}

	balance, unit, isValid := extractBalance(&payload)
	return balance, unit, isValid, nil
}

// recordRefreshFailure 记录刷新失败到 last_refresh_error / is_valid，失败仅记日志。
func (s *ChannelProviderService) recordRefreshFailure(ctx context.Context, normalizedBaseURL, message string) {
	if err := s.providerRepo.UpdateBalance(ctx, normalizedBaseURL, nil, "USD", false, message, time.Now()); err != nil {
		slog.Warn("channel_provider_record_failure_failed",
			"base_url", normalizedBaseURL, "err", err)
	}
}

// usageResponse 是上游 /v1/usage 响应的宽松投影。使用指针字段以区分"未返回"与"零值"。
// 兼容多种常见格式（OneAPI/NewAPI 等）：顶层 remaining、嵌套 quota.remaining、顶层 balance。
type usageResponse struct {
	Remaining *float64      `json:"remaining"`
	Balance   *float64      `json:"balance"`
	Unit      *string       `json:"unit"`
	IsActive  *bool         `json:"is_active"`
	IsValid   *bool         `json:"is_valid"`
	Quota     *quotaBlock   `json:"quota"`
}

type quotaBlock struct {
	Remaining *float64 `json:"remaining"`
	Unit      *string  `json:"unit"`
}

// extractBalance 按固定 fallback 顺序提取余额、单位、有效性。
//   - remaining: resp.Remaining ?? resp.Quota.Remaining ?? resp.Balance
//   - unit:      resp.Unit ?? resp.Quota.Unit ?? "USD"
//   - isValid:   resp.IsActive ?? resp.IsValid ?? true
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

	switch {
	case resp.IsActive != nil:
		isValid = *resp.IsActive
	case resp.IsValid != nil:
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
