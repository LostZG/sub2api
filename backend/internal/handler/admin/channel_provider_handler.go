package admin

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// channelProviderRefreshAllTimeout 限定刷新全部的总体超时。
// 5 并发 × 单次 15s，理论最坏 ~3 轮，120s 留足余量。
const channelProviderRefreshAllTimeout = 120 * time.Second

// ChannelProviderHandler 处理渠道号商（按 baseUrl 聚合的上游渠道商）管理请求。
type ChannelProviderHandler struct {
	providerService *service.ChannelProviderService
}

// NewChannelProviderHandler 构造渠道号商 handler。
func NewChannelProviderHandler(providerService *service.ChannelProviderService) *ChannelProviderHandler {
	return &ChannelProviderHandler{providerService: providerService}
}

// --- Request / Response types ---

type updateProviderRequest struct {
	BaseURL        string  `json:"base_url" binding:"required"`
	RechargeAmount float64 `json:"recharge_amount" binding:"min=0"`
	DisplayName    string  `json:"display_name"`
	QuotaPerUnit   int64   `json:"quota_per_unit"`
}

type refreshProviderRequest struct {
	BaseURL string `json:"base_url" binding:"required"`
}

type channelProviderResponse struct {
	ID               int64   `json:"id"`
	BaseURL          string  `json:"base_url"`
	DisplayName      *string `json:"display_name"`
	RechargeAmount   float64 `json:"recharge_amount"`
	QuotaPerUnit     int64   `json:"quota_per_unit"`
	Balance          *float64 `json:"balance"`
	BalanceUnit      string  `json:"balance_unit"`
	BalanceCheckedAt string  `json:"balance_checked_at"`
	IsValid          bool    `json:"is_valid"`
	SyncBalance      bool    `json:"sync_balance"`
	LastRefreshError string  `json:"last_refresh_error"`
	AccountCount     int64   `json:"account_count"`
	UpdatedAt        string  `json:"updated_at"`
}

func providerToResponse(agg *service.ChannelProviderAggregated) *channelProviderResponse {
	if agg == nil {
		return nil
	}
	resp := &channelProviderResponse{
		ID:             agg.ID,
		BaseURL:        agg.BaseURL,
		DisplayName:    agg.DisplayName,
		RechargeAmount: agg.RechargeAmount,
		QuotaPerUnit:   agg.QuotaPerUnit,
		Balance:        agg.Balance,
		BalanceUnit:    agg.BalanceUnit,
		IsValid:        agg.IsValid,
		SyncBalance:    agg.SyncBalance,
		AccountCount:   agg.AccountCount,
		UpdatedAt:      agg.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if agg.BalanceCheckedAt != nil {
		resp.BalanceCheckedAt = agg.BalanceCheckedAt.Format("2006-01-02T15:04:05Z")
	}
	if agg.LastRefreshError != nil {
		resp.LastRefreshError = *agg.LastRefreshError
	}
	return resp
}

func providerEntityToResponse(p *service.ChannelProvider) *channelProviderResponse {
	if p == nil {
		return nil
	}
	resp := &channelProviderResponse{
		ID:             p.ID,
		BaseURL:        p.BaseURL,
		DisplayName:    p.DisplayName,
		RechargeAmount: p.RechargeAmount,
		QuotaPerUnit:   p.QuotaPerUnit,
		Balance:        p.Balance,
		BalanceUnit:    p.BalanceUnit,
		IsValid:        p.IsValid,
		SyncBalance:    p.SyncBalance,
		UpdatedAt:      p.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if p.BalanceCheckedAt != nil {
		resp.BalanceCheckedAt = p.BalanceCheckedAt.Format("2006-01-02T15:04:05Z")
	}
	if p.LastRefreshError != nil {
		resp.LastRefreshError = *p.LastRefreshError
	}
	return resp
}

// --- Handlers ---

// List 返回渠道号商聚合列表（按 base_url 去重，不含 apiKey）
// GET /api/v1/admin/channel-providers
func (h *ChannelProviderHandler) List(c *gin.Context) {
	aggregated, err := h.providerService.List(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]*channelProviderResponse, 0, len(aggregated))
	for i := range aggregated {
		out = append(out, providerToResponse(&aggregated[i]))
	}
	response.Success(c, out)
}

// UpdateProvider 编辑某个渠道商的可编辑字段（充值金额 / 名称 / quota 系数）
// PUT /api/v1/admin/channel-providers/recharge
func (h *ChannelProviderHandler) UpdateProvider(c *gin.Context) {
	var req updateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	if err := h.providerService.UpdateProvider(c.Request.Context(), req.BaseURL, req.RechargeAmount, req.DisplayName, req.QuotaPerUnit); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "provider updated"})
}

type setSyncBalanceRequest struct {
	BaseURL     string `json:"base_url" binding:"required"`
	SyncBalance bool   `json:"sync_balance"`
}

// SetSyncBalance 切换是否参与"刷新全部"的余额同步
// POST /api/v1/admin/channel-providers/sync-toggle
func (h *ChannelProviderHandler) SetSyncBalance(c *gin.Context) {
	var req setSyncBalanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	if err := h.providerService.SetSyncBalance(c.Request.Context(), req.BaseURL, req.SyncBalance); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "sync_balance updated"})
}

// Refresh 刷新单个渠道商余额，返回更新后的渠道商
// POST /api/v1/admin/channel-providers/refresh
func (h *ChannelProviderHandler) Refresh(c *gin.Context) {
	var req refreshProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	provider, err := h.providerService.RefreshBalance(c.Request.Context(), req.BaseURL)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, providerEntityToResponse(provider))
}

// RefreshAll 刷新全部渠道商余额，返回每个渠道商的成功/失败结果
// POST /api/v1/admin/channel-providers/refresh-all
func (h *ChannelProviderHandler) RefreshAll(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), channelProviderRefreshAllTimeout)
	defer cancel()

	results, err := h.providerService.RefreshAllBalances(ctx)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if results == nil {
		results = []service.RefreshResult{}
	}
	response.Success(c, results)
}
