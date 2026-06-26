/**
 * Admin Channel Providers API
 * 按上游渠道商（baseUrl）维度聚合的管理接口
 */

import { apiClient } from '../client'

export interface ChannelProvider {
  id: number
  base_url: string
  display_name: string | null
  recharge_amount: number
  balance: number | null
  balance_unit: string
  balance_checked_at: string
  is_valid: boolean
  last_refresh_error: string
  account_count: number
  updated_at: string
}

export interface RefreshResult {
  base_url: string
  success: boolean
  message?: string
}

export interface UpdateRechargeAmountRequest {
  base_url: string
  recharge_amount: number
}

export interface RefreshProviderRequest {
  base_url: string
}

/**
 * 渠道号商聚合列表（按 base_url 去重，不含 apiKey）
 */
export async function list(): Promise<ChannelProvider[]> {
  const { data } = await apiClient.get<ChannelProvider[]>('/admin/channel-providers')
  return data ?? []
}

/**
 * 编辑某个渠道商的充值金额
 */
export async function updateRechargeAmount(req: UpdateRechargeAmountRequest): Promise<void> {
  await apiClient.put('/admin/channel-providers/recharge', req)
}

/**
 * 刷新单个渠道商余额，返回更新后的渠道商
 */
export async function refreshBalance(req: RefreshProviderRequest): Promise<ChannelProvider> {
  const { data } = await apiClient.post<ChannelProvider>('/admin/channel-providers/refresh', req)
  return data
}

/**
 * 刷新全部渠道商余额。后端最多 5 并发 × 15s，整体超时 120s，
 * 因此前端单独放宽该请求的超时（默认 30s 可能不够）。
 */
export async function refreshAllBalances(): Promise<RefreshResult[]> {
  const { data } = await apiClient.post<RefreshResult[]>('/admin/channel-providers/refresh-all', {}, {
    timeout: 120000
  })
  return data ?? []
}

const channelProvidersAPI = { list, updateRechargeAmount, refreshBalance, refreshAllBalances }
export default channelProvidersAPI
