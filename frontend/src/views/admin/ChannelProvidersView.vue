<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.channelProviders.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.channelProviders.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              @click="loadProviders"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh', 'Refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              @click="handleRefreshAll"
              :disabled="refreshingAll"
              class="btn btn-primary"
            >
              <Icon name="refresh" size="md" class="mr-2" :class="refreshingAll ? 'animate-spin' : ''" />
              {{ t('admin.channelProviders.refreshAll') }}
            </button>
          </div>
        </div>
      </template>

      <template #filters>
        <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
          {{ errorMessage }}
        </div>
        <div v-if="successMessage" class="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300">
          {{ successMessage }}
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="providers"
          :loading="loading"
        >
          <template #empty>
            <EmptyState :message="t('admin.channelProviders.noProviders')" />
          </template>

          <template #cell-base_url="{ value }">
            <span class="font-medium text-gray-900 dark:text-white break-all">{{ value }}</span>
          </template>

          <template #cell-display_name="{ row }">
            <input
              type="text"
              maxlength="200"
              class="input w-36 py-1"
              :value="editingValue(row.base_url, nameDrafts, row.display_name ?? '')"
              :disabled="savingBaseUrl === row.base_url"
              @input="setEditingValue(row.base_url, nameDrafts, $event)"
              @blur="saveProvider(row)"
              @keydown.enter.prevent="saveProvider(row)"
            />
          </template>

          <template #cell-recharge_amount="{ row }">
            <div class="flex items-center gap-1">
              <span class="text-gray-400">¥</span>
              <input
                type="number"
                min="0"
                step="0.01"
                class="input w-28 py-1"
                :value="editingValue(row.base_url, amountDrafts, String(row.recharge_amount ?? 0))"
                :disabled="savingBaseUrl === row.base_url"
                @input="setEditingValue(row.base_url, amountDrafts, $event)"
                @blur="saveProvider(row)"
                @keydown.enter.prevent="saveProvider(row)"
              />
              <span
                v-if="savingBaseUrl === row.base_url"
                class="ml-1 text-xs text-gray-400"
              >{{ t('common.saving', '...') }}</span>
            </div>
          </template>

          <template #cell-quota_per_unit="{ row }">
            <input
              type="number"
              min="1"
              step="1"
              class="input w-28 py-1"
              :value="editingValue(row.base_url, quotaDrafts, String(row.quota_per_unit ?? 500000))"
              :disabled="savingBaseUrl === row.base_url"
              @input="setEditingValue(row.base_url, quotaDrafts, $event)"
              @blur="saveProvider(row)"
              @keydown.enter.prevent="saveProvider(row)"
            />
            <div class="mt-0.5 text-xs text-gray-400">{{ t('admin.channelProviders.quotaHint') }}</div>
          </template>

          <template #cell-balance="{ row }">
            <div class="flex flex-col">
              <span
                :class="balanceColorClass(row)"
                class="font-medium"
              >
                <template v-if="row.balance !== null && row.balance !== undefined">
                  {{ Number(row.balance).toFixed(4) }} {{ row.balance_unit }}
                </template>
                <template v-else>-</template>
              </span>
              <span
                v-if="row.last_refresh_error"
                class="mt-0.5 max-w-xs truncate text-xs text-red-500 dark:text-red-400"
                :title="row.last_refresh_error"
              >
                {{ row.last_refresh_error }}
              </span>
            </div>
          </template>

          <template #cell-balance_checked_at="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-account_count="{ value }">
            <span
              class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-800 dark:bg-dark-600 dark:text-gray-300"
            >
              {{ value }}
            </span>
          </template>

          <template #cell-sync_balance="{ row }">
            <Toggle
              :modelValue="row.sync_balance"
              @update:modelValue="(v: boolean) => handleSyncToggle(row, v)"
            />
          </template>

          <template #cell-actions="{ row }">
            <button
              @click="handleRefreshOne(row.base_url)"
              :disabled="loadingBaseUrl === row.base_url || refreshingAll"
              class="btn btn-secondary btn-sm"
              :title="t('admin.channelProviders.refresh')"
            >
              <Icon
                name="refresh"
                size="md"
                :class="loadingBaseUrl === row.base_url ? 'animate-spin' : ''"
              />
            </button>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { ChannelProvider, RefreshResult } from '@/api/admin/channelProviders'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const providers = ref<ChannelProvider[]>([])
const loading = ref(false)
const loadingBaseUrl = ref<string>('')
const refreshingAll = ref(false)
const savingBaseUrl = ref<string>('')
const errorMessage = ref('')
const successMessage = ref('')

// 行内编辑草稿：base_url → 草稿值。仅记录用户改动过的字段，未改动的回填原始值。
// 后端 updateProvider 是全字段更新，因此保存时三字段一起提交（取草稿或原值）。
const amountDrafts = reactive<Record<string, string>>({})
const nameDrafts = reactive<Record<string, string>>({})
const quotaDrafts = reactive<Record<string, string>>({})

const columns = computed<Column[]>(() => [
  { key: 'base_url', label: t('admin.channelProviders.columns.baseUrl'), sortable: true },
  { key: 'display_name', label: t('admin.channelProviders.columns.displayName'), sortable: false },
  { key: 'recharge_amount', label: t('admin.channelProviders.columns.rechargeAmount'), sortable: false },
  { key: 'quota_per_unit', label: t('admin.channelProviders.columns.quotaPerUnit'), sortable: false },
  { key: 'balance', label: t('admin.channelProviders.columns.balance'), sortable: false },
  { key: 'balance_checked_at', label: t('admin.channelProviders.columns.balanceCheckedAt'), sortable: false },
  { key: 'account_count', label: t('admin.channelProviders.columns.accountCount'), sortable: false },
  { key: 'sync_balance', label: t('admin.channelProviders.columns.syncBalance'), sortable: true },
  { key: 'actions', label: t('admin.channelProviders.columns.actions'), sortable: false }
])

function editingValue(baseUrl: string, draftMap: Record<string, string>, fallback: string): string {
  if (baseUrl in draftMap) return draftMap[baseUrl]
  return fallback
}

function setEditingValue(baseUrl: string, draftMap: Record<string, string>, ev: Event) {
  const target = ev.target as HTMLInputElement
  draftMap[baseUrl] = target.value
}

async function loadProviders() {
  loading.value = true
  errorMessage.value = ''
  try {
    providers.value = await adminAPI.channelProviders.list()
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err)
  } finally {
    loading.value = false
  }
}

// saveProvider 在任一字段失焦时触发：若三字段任一改动，全字段提交。
async function saveProvider(row: ChannelProvider) {
  const amountDraft = amountDrafts[row.base_url]
  const nameDraft = nameDrafts[row.base_url]
  const quotaDraft = quotaDrafts[row.base_url]

  // 三字段都没改动则跳过
  if (amountDraft === undefined && nameDraft === undefined && quotaDraft === undefined) return

  const amount = amountDraft !== undefined ? Number(amountDraft) : Number(row.recharge_amount ?? 0)
  if (!Number.isFinite(amount) || amount < 0) {
    errorMessage.value = t('admin.channelProviders.invalidAmount')
    return
  }
  const quota = quotaDraft !== undefined ? Number(quotaDraft) : Number(row.quota_per_unit ?? 500000)
  if (!Number.isFinite(quota) || quota <= 0) {
    errorMessage.value = t('admin.channelProviders.invalidQuota')
    return
  }
  const name = nameDraft !== undefined ? nameDraft : (row.display_name ?? '')

  // 值都没变则跳过
  const nameChanged = name !== (row.display_name ?? '')
  const amountChanged = amount !== Number(row.recharge_amount ?? 0)
  const quotaChanged = quota !== Number(row.quota_per_unit ?? 500000)
  if (!nameChanged && !amountChanged && !quotaChanged) {
    delete amountDrafts[row.base_url]
    delete nameDrafts[row.base_url]
    delete quotaDrafts[row.base_url]
    return
  }

  savingBaseUrl.value = row.base_url
  errorMessage.value = ''
  try {
    await adminAPI.channelProviders.updateProvider({
      base_url: row.base_url,
      recharge_amount: amount,
      display_name: name,
      quota_per_unit: quota
    })
    row.recharge_amount = amount
    row.display_name = name || null
    row.quota_per_unit = quota
    delete amountDrafts[row.base_url]
    delete nameDrafts[row.base_url]
    delete quotaDrafts[row.base_url]
    successMessage.value = t('admin.channelProviders.saveSuccess')
    setTimeout(() => { successMessage.value = '' }, 2500)
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err)
  } finally {
    savingBaseUrl.value = ''
  }
}

// handleSyncToggle 切换"是否参与刷新全部"。乐观更新，失败回滚。
async function handleSyncToggle(row: ChannelProvider, enabled: boolean) {
  const prev = row.sync_balance
  row.sync_balance = enabled
  errorMessage.value = ''
  try {
    await adminAPI.channelProviders.setSyncBalance(row.base_url, enabled)
    successMessage.value = t('admin.channelProviders.saveSuccess')
    setTimeout(() => { successMessage.value = '' }, 2500)
  } catch (err) {
    row.sync_balance = prev
    errorMessage.value = extractApiErrorMessage(err)
  }
}

async function handleRefreshOne(baseUrl: string) {
  loadingBaseUrl.value = baseUrl
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const updated = await adminAPI.channelProviders.refreshBalance({ base_url: baseUrl })
    const idx = providers.value.findIndex(p => p.base_url === baseUrl)
    if (idx >= 0) {
      // 单行刷新返回的 provider 不含 account_count（后端 GetByBaseURL 无此字段），
      // 仅更新余额相关字段，避免覆盖原有的 account_count。
      const cur = providers.value[idx]
      providers.value[idx] = {
        ...cur,
        balance: updated.balance,
        balance_unit: updated.balance_unit,
        balance_checked_at: updated.balance_checked_at,
        is_valid: updated.is_valid,
        last_refresh_error: updated.last_refresh_error
      }
    }
    successMessage.value = t('admin.channelProviders.refreshSuccess')
    setTimeout(() => { successMessage.value = '' }, 2500)
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err)
  } finally {
    loadingBaseUrl.value = ''
  }
}

async function handleRefreshAll() {
  refreshingAll.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const results: RefreshResult[] = await adminAPI.channelProviders.refreshAllBalances()
    const ok = results.filter(r => r.success && !r.skipped).length
    const skipped = results.filter(r => r.skipped).length
    const failed = results.filter(r => !r.success).length
    // 重新加载以拿到最新余额
    await loadProviders()
    if (failed === 0 && skipped === 0) {
      successMessage.value = t('admin.channelProviders.refreshSuccess')
    } else {
      successMessage.value = t('admin.channelProviders.refreshAllSummary', {
        ok: String(ok),
        failed: String(failed),
        skipped: String(skipped)
      })
    }
    setTimeout(() => { successMessage.value = '' }, 4000)
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err)
  } finally {
    refreshingAll.value = false
  }
}

// 余额颜色：≤3 红、3~10 黄、≥10 绿；刷新失败或无余额数据用对应语义色
function balanceColorClass(row: ChannelProvider): string {
  if (!row.is_valid) return 'text-red-600 dark:text-red-400'
  if (row.balance === null || row.balance === undefined) return 'text-gray-400'
  const bal = Number(row.balance)
  if (isNaN(bal)) return 'text-gray-400'
  if (bal <= 3) return 'text-red-600 dark:text-red-400'
  if (bal < 10) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-green-600 dark:text-green-400'
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

onMounted(() => {
  loadProviders()
})
</script>
