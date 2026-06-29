<template>
  <BaseDialog :show="show" :title="dialogTitle" width="extra-wide" @close="handleClose">
    <div class="space-y-4">
      <!-- 顶部：刷新分组倍率 + 上次刷新时间 -->
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="text-sm text-gray-500 dark:text-gray-400">
          <span>{{ t('admin.channelProviders.accountsDialog.lastRefresh') }}: </span>
          <span class="font-medium text-gray-700 dark:text-gray-200">{{ formatDateTime(groupRatioCheckedAt) }}</span>
        </div>
        <button
          @click="handleRefreshRatio"
          :disabled="refreshingRatio || loading"
          class="btn btn-secondary btn-sm"
        >
          <Icon
            name="refresh"
            size="md"
            class="mr-2"
            :class="refreshingRatio ? 'animate-spin' : ''"
          />
          {{ t('admin.channelProviders.accountsDialog.refreshGroupRatio') }}
        </button>
      </div>

      <!-- 状态提示 -->
      <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 px-4 py-2 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
        {{ errorMessage }}
      </div>
      <div v-if="successMessage" class="rounded-lg border border-green-200 bg-green-50 px-4 py-2 text-sm text-green-700 dark:border-green-900 dark:bg-green-950 dark:text-green-300">
        {{ successMessage }}
      </div>

      <!-- 账号表格 -->
      <div v-if="loading" class="flex justify-center py-10">
        <Icon name="refresh" size="md" class="animate-spin text-gray-400" />
      </div>
      <DataTable
        v-else
        :columns="columns"
        :data="accounts"
      >
        <template #empty>
          <EmptyState :message="t('admin.channelProviders.noProviders')" />
        </template>

        <template #cell-name="{ value }">
          <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
        </template>

        <template #cell-platform="{ value }">
          <span class="text-sm text-gray-600 dark:text-gray-300">{{ value }}</span>
        </template>

        <template #cell-status="{ value }">
          <span :class="statusBadgeClass(value)" class="badge text-xs">{{ value }}</span>
        </template>

        <template #cell-rate_multiplier="{ value }">
          <span class="text-sm text-gray-600 dark:text-gray-300">{{ Number(value).toFixed(3) }}</span>
        </template>

        <template #cell-last_used_at="{ value }">
          <span class="text-sm text-gray-600 dark:text-gray-300">{{ formatDateTime(value) }}</span>
        </template>

        <template #cell-upstream_group="{ value }">
          <span v-if="value" class="inline-flex items-center rounded bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-800 dark:bg-dark-600 dark:text-gray-300">{{ value }}</span>
          <span v-else class="text-gray-400">-</span>
        </template>

        <template #cell-latest_ratio="{ row }">
          <span v-if="latestRatio(row) !== null" class="font-medium text-gray-900 dark:text-white">{{ latestRatio(row) }}</span>
          <span v-else class="text-gray-400" :title="ratioTooltip(row)">-</span>
        </template>
      </DataTable>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ProviderAccountBrief } from '@/types'
import type { Column } from '@/components/common/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{ show: boolean; baseURL: string | null }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()

const accounts = ref<ProviderAccountBrief[]>([])
const groupRatio = ref<Record<string, number>>({})
const groupRatioCheckedAt = ref('')
const loading = ref(false)
const refreshingRatio = ref(false)
const errorMessage = ref('')
const successMessage = ref('')

const dialogTitle = computed(() =>
  t('admin.channelProviders.accountsDialog.title', { name: props.baseURL ?? '' })
)

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channelProviders.accountsDialog.columns.name'), sortable: true },
  { key: 'id', label: t('admin.channelProviders.accountsDialog.columns.id'), sortable: true },
  { key: 'platform', label: t('admin.channelProviders.accountsDialog.columns.platform'), sortable: true },
  { key: 'status', label: t('admin.channelProviders.accountsDialog.columns.status'), sortable: true },
  { key: 'priority', label: t('admin.channelProviders.accountsDialog.columns.priority'), sortable: true },
  { key: 'rate_multiplier', label: t('admin.channelProviders.accountsDialog.columns.rateMultiplier'), sortable: true },
  { key: 'last_used_at', label: t('admin.channelProviders.accountsDialog.columns.lastUsedAt'), sortable: true },
  { key: 'upstream_group', label: t('admin.channelProviders.accountsDialog.columns.upstreamGroup'), sortable: true },
  { key: 'latest_ratio', label: t('admin.channelProviders.accountsDialog.columns.latestRatio'), sortable: false }
])

function latestRatio(row: ProviderAccountBrief): number | null {
  if (!row.upstream_group) return null
  const v = groupRatio.value[row.upstream_group]
  if (v === undefined || v === null) return null
  return v
}

function ratioTooltip(row: ProviderAccountBrief): string {
  if (!row.upstream_group) return t('admin.channelProviders.accountsDialog.noUpstreamGroup')
  return t('admin.channelProviders.accountsDialog.noRatio')
}

function statusBadgeClass(status: string): string {
  if (status === 'active') return 'badge-success'
  if (status === 'error') return 'badge-danger'
  return 'badge-gray'
}

function formatDateTime(value: string): string {
  if (!value) return '-'
  const d = new Date(value)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

async function load() {
  if (!props.baseURL) return
  loading.value = true
  errorMessage.value = ''
  try {
    const res = await adminAPI.channelProviders.listAccounts(props.baseURL)
    accounts.value = res.accounts || []
    groupRatio.value = res.group_ratio || {}
    groupRatioCheckedAt.value = res.group_ratio_checked_at || ''
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err)
    accounts.value = []
  } finally {
    loading.value = false
  }
}

async function handleRefreshRatio() {
  if (!props.baseURL) return
  refreshingRatio.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const updated = await adminAPI.channelProviders.refreshGroupRatio(props.baseURL)
    groupRatio.value = updated.group_ratio || {}
    groupRatioCheckedAt.value = updated.group_ratio_checked_at || ''
    successMessage.value = t('admin.channelProviders.accountsDialog.refreshSuccess')
    setTimeout(() => { successMessage.value = '' }, 2500)
  } catch (err) {
    errorMessage.value = extractApiErrorMessage(err) || t('admin.channelProviders.accountsDialog.refreshFailed')
  } finally {
    refreshingRatio.value = false
  }
}

function handleClose() {
  emit('close')
}

watch(
  () => [props.show, props.baseURL] as const,
  ([show, baseURL]) => {
    if (show && baseURL) {
      load()
    } else {
      accounts.value = []
      groupRatio.value = {}
      groupRatioCheckedAt.value = ''
      errorMessage.value = ''
      successMessage.value = ''
    }
  },
  { immediate: true }
)
</script>
