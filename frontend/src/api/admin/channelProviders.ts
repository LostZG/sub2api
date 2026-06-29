/**
 * Admin Channel Providers API
 * 按上游渠道商（baseUrl）维度聚合的管理接口
 */

import { apiClient } from '../client'
import type { ProviderAccountBrief } from '@/types'

export interface ChannelProvider {
  id: number
  base_url: string
  display_name: string | null
  recharge_amount: number
  quota_per_unit: number
  balance: number | null
  balance_unit: string
  balance_checked_at: string
  is_valid: boolean
  sync_balance: boolean
  last_refresh_error: string
  group_ratio?: Record<string, number>
  group_ratio_checked_at?: string
  account_count: number
  updated_at: string
}

export interface ProviderAccountsResponse {
  accounts: ProviderAccountBrief[]
  group_ratio: Record<string, number>
  group_ratio_checked_at: string
}

export interface RefreshResult {
  base_url: string
  success: boolean
  skipped?: boolean
  message?: string
}

export interface UpdateProviderRequest {
  base_url: string
  recharge_amount: number
  display_name: string
  quota_per_unit: number
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
 * 更新某个渠道商的可编辑字段（充值金额 / 名称 / quota 系数）
 */
export async function updateProvider(req: UpdateProviderRequest): Promise<void> {
  await apiClient.put('/admin/channel-providers/recharge', req)
}

/**
 * 切换是否参与"刷新全部"的余额同步
 */
export async function setSyncBalance(baseURL: string, enabled: boolean): Promise<void> {
  await apiClient.post('/admin/channel-providers/sync-toggle', {
    base_url: baseURL,
    sync_balance: enabled
  })
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

/**
 * 拉取某渠道商下所有账号摘要 + 分组倍率缓存（不含 credentials）
 */
export async function listAccounts(baseURL: string): Promise<ProviderAccountsResponse> {
  const { data } = await apiClient.get<ProviderAccountsResponse>('/admin/channel-providers/accounts', {
    params: { base_url: baseURL }
  })
  return data ?? { accounts: [], group_ratio: {}, group_ratio_checked_at: '' }
}

/**
 * 刷新上游 NewAPI 分组倍率（/api/pricing），返回更新后的渠道商
 */
export async function refreshGroupRatio(baseURL: string): Promise<ChannelProvider> {
  const { data } = await apiClient.post<ChannelProvider>(
    '/admin/channel-providers/refresh-group-ratio',
    { base_url: baseURL },
    { timeout: 30000 }
  )
  return data
}

const channelProvidersAPI = {
  list,
  updateProvider,
  setSyncBalance,
  refreshBalance,
  refreshAllBalances,
  listAccounts,
  refreshGroupRatio
}
export default channelProvidersAPI
