<template>
  <section data-test="upstream-management-panel" class="space-y-4">
    <div class="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
      <div>
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.customFeatures.upstream.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.customFeatures.upstream.description') }}
        </p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary inline-flex items-center gap-2"
          :disabled="syncingAll"
          @click="syncAllSites"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': syncingAll }" />
          {{ t('admin.customFeatures.upstream.syncAll') }}
        </button>
        <button type="button" class="btn btn-primary inline-flex items-center gap-2" data-test="upstream-add" @click="openCreate">
          <Icon name="plus" size="sm" />
          {{ t('admin.customFeatures.upstream.add') }}
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_160px_160px]">
      <div class="relative">
        <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" />
        <input
          v-model="search"
          type="search"
          class="input pl-9"
          :placeholder="t('admin.customFeatures.upstream.searchPlaceholder')"
          @keyup.enter="applyFilters"
        />
      </div>
      <select v-model="platformFilter" class="input" :aria-label="t('admin.customFeatures.upstream.platform')" @change="applyFilters">
        <option value="">{{ t('admin.customFeatures.upstream.allPlatforms') }}</option>
        <option value="sub2api">Sub2API</option>
        <option value="newapi">New API</option>
      </select>
      <select v-model="enabledFilter" class="input" :aria-label="t('admin.customFeatures.upstream.enabledState')" @change="applyFilters">
        <option value="">{{ t('admin.customFeatures.upstream.allStates') }}</option>
        <option value="true">{{ t('admin.customFeatures.upstream.enabled') }}</option>
        <option value="false">{{ t('admin.customFeatures.upstream.disabled') }}</option>
      </select>
    </div>

    <div v-if="loadError" class="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/20 dark:text-red-300">
      <div class="flex items-center justify-between gap-3">
        <span>{{ loadError }}</span>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="loadSites(false)">
          <Icon name="refresh" size="sm" />
          {{ t('common.tryAgain') }}
        </button>
      </div>
    </div>

    <DataTable
      v-else
      :columns="columns"
      :data="sites"
      :loading="loading"
      row-key="id"
      :sticky-first-column="true"
      :sticky-actions-column="true"
      :actions-count="6"
      :expanded-row-keys="expandedSiteKeys"
    >
      <template #cell-site="{ row }">
        <div class="flex max-w-64 items-start gap-1 whitespace-normal">
          <button
            type="button"
            class="icon-action mt-[-0.25rem] flex-shrink-0"
            :title="isSiteExpanded(row.id) ? t('admin.customFeatures.upstream.hideGroups') : t('admin.customFeatures.upstream.showGroups')"
            :aria-expanded="isSiteExpanded(row.id)"
            :aria-controls="`upstream-groups-${row.id}`"
            :data-test="`upstream-expand-${row.id}`"
            @click.stop="toggleSiteGroups(row)"
          >
            <Icon :name="isSiteExpanded(row.id) ? 'chevronDown' : 'chevronRight'" size="sm" />
          </button>
          <div class="min-w-0">
            <a
              :href="row.base_url"
              target="_blank"
              rel="noopener noreferrer"
              class="inline-flex items-center gap-1 font-medium text-primary-600 hover:underline dark:text-primary-400"
            >
              <span class="break-words">{{ row.name }}</span>
              <Icon name="externalLink" size="xs" class="flex-shrink-0" />
            </a>
            <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400" :title="row.base_url">{{ row.base_url }}</p>
          </div>
        </div>
      </template>
      <template #cell-platform="{ row }">
        <span class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700 dark:bg-dark-700 dark:text-gray-200">
          {{ platformLabel(row.platform) }}
        </span>
      </template>
      <template #cell-status="{ row }">
        <div class="flex flex-col items-start gap-1">
          <span :class="statusClass(row.status)" class="rounded px-2 py-1 text-xs font-medium" :title="row.error_message || undefined">
            {{ t(`admin.customFeatures.upstream.status.${row.status}`) }}
          </span>
          <span class="text-xs" :class="row.enabled ? 'text-emerald-600 dark:text-emerald-400' : 'text-gray-400'">
            {{ row.enabled ? t('admin.customFeatures.upstream.enabled') : t('admin.customFeatures.upstream.disabled') }}
          </span>
        </div>
      </template>
      <template #cell-balance_usd="{ row }">
        <span class="font-medium">{{ formatMoney(row.balance_usd) }}</span>
      </template>
      <template #cell-today="{ row }">
        <div class="space-y-1 text-xs">
          <p class="font-medium text-gray-800 dark:text-gray-200" :title="formatExactTokens(row.today_tokens)">{{ formatTokens(row.today_tokens) }}</p>
          <p class="text-gray-500 dark:text-gray-400">{{ formatMoney(row.today_cost_usd) }}</p>
        </div>
      </template>
      <template #cell-total="{ row }">
        <div class="space-y-1 text-xs">
          <p class="font-medium text-gray-800 dark:text-gray-200" :title="formatExactTokens(row.total_tokens)">{{ formatTokens(row.total_tokens) }}</p>
          <p class="text-gray-500 dark:text-gray-400">{{ formatMoney(row.total_cost_usd) }}</p>
        </div>
      </template>
      <template #cell-last_synced_at="{ row }">
        <span class="text-xs text-gray-600 dark:text-gray-300">{{ formatDateTime(row.last_synced_at) }}</span>
      </template>
      <template #cell-actions="{ row }">
        <div class="flex flex-wrap items-center justify-end gap-1" @click.stop>
          <a :href="row.base_url" target="_blank" rel="noopener noreferrer" class="icon-action" :title="t('admin.customFeatures.upstream.openSite')">
            <Icon name="externalLink" size="sm" />
          </a>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.sync')" :disabled="row.status === 'syncing'" @click="syncSite(row)">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': row.status === 'syncing' }" />
          </button>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.details')" @click="openDetails(row)">
            <Icon name="chart" size="sm" />
          </button>
          <button type="button" class="icon-action" :title="t('admin.customFeatures.upstream.edit')" @click="openEdit(row)">
            <Icon name="edit" size="sm" />
          </button>
          <button type="button" class="icon-action" :title="row.enabled ? t('admin.customFeatures.upstream.disable') : t('admin.customFeatures.upstream.enable')" @click="toggleSite(row)">
            <Icon :name="row.enabled ? 'ban' : 'play'" size="sm" />
          </button>
          <button type="button" class="icon-action text-red-600 hover:text-red-700 dark:text-red-400" :title="t('admin.customFeatures.upstream.delete')" @click="deleteTarget = row">
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </template>
      <template #row-details="{ row }">
        <section :id="`upstream-groups-${row.id}`" class="bg-gray-50/80 px-1 py-3 dark:bg-dark-800/40" :data-test="`upstream-groups-${row.id}`">
          <div v-if="groupState(row.id).loading" class="flex min-h-24 items-center justify-center">
            <span class="h-6 w-6 animate-spin rounded-full border-b-2 border-primary-600"></span>
          </div>
          <div v-else-if="groupState(row.id).error" class="flex min-h-24 flex-col items-center justify-center gap-3 text-center text-sm text-red-600 dark:text-red-300">
            <span>{{ groupState(row.id).error }}</span>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :data-test="`upstream-groups-retry-${row.id}`" @click="loadSiteGroups(row, true)">
              <Icon name="refresh" size="sm" />
              {{ t('common.tryAgain') }}
            </button>
          </div>
          <p v-else-if="groupState(row.id).groups.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.customFeatures.upstream.noGroups') }}
          </p>
          <div v-else class="grid grid-cols-1 md:grid-cols-2 md:gap-3" data-test="expanded-groups-grid">
            <article
              v-for="group in groupState(row.id).groups"
              :key="group.remote_id"
              class="border-b border-gray-200 py-4 last:border-b-0 dark:border-dark-700 md:rounded-md md:border md:bg-white md:p-4 md:last:border-b md:dark:bg-dark-900"
            >
              <div class="flex min-w-0 items-start justify-between gap-3">
                <div class="min-w-0">
                  <h4 class="break-words text-sm font-semibold text-gray-900 dark:text-gray-100" :title="group.name">{{ group.name }}</h4>
                  <p v-if="group.description" class="mt-1 line-clamp-2 break-words text-xs text-gray-500 dark:text-gray-400" :title="group.description">{{ group.description }}</p>
                </div>
                <span class="flex-shrink-0 rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ group.platform || '—' }}</span>
              </div>
              <dl class="mt-4 grid grid-cols-3 gap-3">
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.multiplier') }}</dt>
                  <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatMultiplier(group.multiplier) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.tokens') }}</dt>
                  <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100" :title="formatExactTokens(group.today_tokens)">{{ formatTokens(group.today_tokens) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.customFeatures.upstream.groupColumns.cost') }}</dt>
                  <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-gray-100">{{ formatMoney(group.today_cost_usd) }}</dd>
                </div>
              </dl>
            </article>
          </div>
        </section>
      </template>
      <template #empty>
        <div class="flex flex-col items-center py-4">
          <Icon name="server" size="xl" class="mb-3 text-gray-400" />
          <p class="font-medium text-gray-800 dark:text-gray-200">{{ t('admin.customFeatures.upstream.empty') }}</p>
        </div>
      </template>
    </DataTable>

    <div v-if="total > pageSize" class="flex items-center justify-between gap-4 text-sm text-gray-500 dark:text-gray-400">
      <span>{{ t('admin.customFeatures.upstream.pagination', { page, pages, total }) }}</span>
      <div class="flex gap-2">
        <button type="button" class="btn btn-secondary" :disabled="page <= 1" @click="changePage(page - 1)">{{ t('admin.customFeatures.upstream.previous') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="page >= pages" @click="changePage(page + 1)">{{ t('admin.customFeatures.upstream.next') }}</button>
      </div>
    </div>

    <BaseDialog :show="formOpen" :title="editingSite ? t('admin.customFeatures.upstream.editTitle') : t('admin.customFeatures.upstream.addTitle')" width="wide" @close="closeForm">
      <form class="grid grid-cols-1 gap-5 md:grid-cols-2" data-test="upstream-form" @submit.prevent="submitForm">
        <div>
          <label for="upstream-name" class="input-label">{{ t('admin.customFeatures.upstream.name') }}</label>
          <input id="upstream-name" v-model="form.name" class="input" required maxlength="100" />
        </div>
        <div>
          <label for="upstream-url" class="input-label">{{ t('admin.customFeatures.upstream.baseUrl') }}</label>
          <input id="upstream-url" v-model="form.base_url" class="input" type="url" required placeholder="https://example.com" />
        </div>
        <div>
          <label for="upstream-platform" class="input-label">{{ t('admin.customFeatures.upstream.platform') }}</label>
          <select id="upstream-platform" v-model="form.platform" class="input" @change="handlePlatformChange">
            <option value="sub2api">Sub2API</option>
            <option value="newapi">New API</option>
          </select>
        </div>
        <div>
          <label for="upstream-auth-mode" class="input-label">{{ t('admin.customFeatures.upstream.authMode') }}</label>
          <select id="upstream-auth-mode" v-model="form.auth_mode" class="input" :disabled="form.platform === 'newapi'">
            <option value="password">{{ t('admin.customFeatures.upstream.passwordAuth') }}</option>
            <option v-if="form.platform === 'sub2api'" value="token">{{ t('admin.customFeatures.upstream.tokenAuth') }}</option>
          </select>
        </div>
        <div v-if="form.auth_mode === 'password'">
          <label for="upstream-account" class="input-label">{{ t('admin.customFeatures.upstream.account') }}</label>
          <input id="upstream-account" v-model="form.account" class="input" autocomplete="username" required />
        </div>
        <div v-if="form.auth_mode === 'password'">
          <label for="upstream-password" class="input-label">{{ t('admin.customFeatures.upstream.password') }}</label>
          <input id="upstream-password" v-model="form.password" class="input" type="password" autocomplete="new-password" :required="!editingSite" :placeholder="editingSite ? t('admin.customFeatures.upstream.keepCredential') : ''" />
        </div>
        <template v-else>
          <div>
            <label for="upstream-access-token" class="input-label">{{ t('admin.customFeatures.upstream.accessToken') }}</label>
            <input id="upstream-access-token" v-model="form.access_token" class="input" type="password" autocomplete="off" :placeholder="editingSite ? t('admin.customFeatures.upstream.keepCredential') : ''" />
          </div>
          <div>
            <label for="upstream-refresh-token" class="input-label">{{ t('admin.customFeatures.upstream.refreshToken') }}</label>
            <input id="upstream-refresh-token" v-model="form.refresh_token" class="input" type="password" autocomplete="off" :placeholder="editingSite ? t('admin.customFeatures.upstream.keepCredential') : ''" />
          </div>
        </template>
        <div class="flex items-center justify-between gap-4 md:col-span-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('admin.customFeatures.upstream.enabled') }}</span>
          <Toggle v-model="form.enabled" />
        </div>
        <div class="flex justify-end gap-2 border-t border-gray-100 pt-4 dark:border-dark-700 md:col-span-2">
          <button type="button" class="btn btn-secondary" @click="closeForm">{{ t('admin.customFeatures.upstream.cancel') }}</button>
          <button type="submit" class="btn btn-primary inline-flex items-center gap-2" :disabled="saving">
            <span v-if="saving" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
            <Icon v-else name="check" size="sm" />
            {{ saving ? t('admin.customFeatures.upstream.validating') : t('admin.customFeatures.upstream.save') }}
          </button>
        </div>
      </form>
    </BaseDialog>

    <BaseDialog :show="Boolean(detailSite)" :title="detailSite?.name || ''" width="extra-wide" @close="closeDetails">
      <div class="space-y-5">
        <div class="flex overflow-x-auto border-b border-gray-200 dark:border-dark-700">
          <button v-for="tab in detailTabs" :key="tab" type="button" class="flex-shrink-0 border-b-2 px-4 py-2 text-sm font-medium" :class="detailTab === tab ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" @click="selectDetailTab(tab)">
            {{ t(`admin.customFeatures.upstream.detailTabs.${tab}`) }}
          </button>
        </div>
        <div v-if="detailLoading" class="flex h-56 items-center justify-center">
          <span class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></span>
        </div>
        <template v-else-if="detailTab === 'groups'">
          <DataTable :columns="groupColumns" :data="detailGroups" row-key="remote_id" :sticky-actions-column="false" :expandable-actions="false">
            <template #cell-name="{ row }">
              <div class="max-w-72 whitespace-normal">
                <p class="font-medium" :title="row.name">{{ row.name }}</p>
                <p v-if="row.description" class="mt-1 line-clamp-2 break-words text-xs text-gray-500 dark:text-gray-400" :title="row.description">{{ row.description }}</p>
              </div>
            </template>
            <template #cell-multiplier="{ row }">{{ formatMultiplier(row.multiplier) }}</template>
            <template #cell-today_tokens="{ row }"><span :title="formatExactTokens(row.today_tokens)">{{ formatTokens(row.today_tokens) }}</span></template>
            <template #cell-today_cost_usd="{ row }">{{ formatMoney(row.today_cost_usd) }}</template>
            <template #empty><p class="py-8 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noGroups') }}</p></template>
          </DataTable>
        </template>
        <template v-else-if="detailTab === 'usage'">
          <div class="flex justify-end">
            <div class="inline-flex rounded-md border border-gray-200 p-1 dark:border-dark-600">
              <button v-for="days in historyRanges" :key="days" type="button" class="min-w-14 rounded px-3 py-1.5 text-sm" :class="historyDays === days ? 'bg-primary-600 text-white' : 'text-gray-600 dark:text-gray-300'" @click="changeHistoryRange(days)">{{ days }}d</button>
            </div>
          </div>
          <div v-if="detailHistory.length" class="h-72">
            <Line :data="historyChartData" :options="historyChartOptions" />
          </div>
          <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noHistory') }}</p>
        </template>
        <template v-else>
          <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div class="min-w-0 flex-1 sm:max-w-sm">
              <label for="upstream-multiplier-group" class="input-label">{{ t('admin.customFeatures.upstream.multiplierGroup') }}</label>
              <select id="upstream-multiplier-group" v-model="selectedMultiplierRemoteID" class="input" :disabled="multiplierLoading || multiplierHistories.length === 0">
                <option v-for="group in multiplierHistories" :key="group.remote_id" :value="group.remote_id">{{ group.name }}</option>
              </select>
            </div>
            <div class="inline-flex self-start rounded-md border border-gray-200 p-1 dark:border-dark-600 sm:self-auto">
              <button v-for="days in historyRanges" :key="days" type="button" class="min-w-14 rounded px-3 py-1.5 text-sm" :class="multiplierDays === days ? 'bg-primary-600 text-white' : 'text-gray-600 dark:text-gray-300'" @click="changeMultiplierRange(days)">{{ days }}d</button>
            </div>
          </div>
          <div v-if="multiplierLoading" class="flex h-56 items-center justify-center">
            <span class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></span>
          </div>
          <div v-else-if="multiplierError" class="flex h-56 flex-col items-center justify-center gap-3 text-sm text-red-600 dark:text-red-300">
            <span>{{ multiplierError }}</span>
            <button type="button" class="btn btn-secondary inline-flex items-center gap-2" data-test="multiplier-history-retry" @click="loadMultiplierHistory(true)">
              <Icon name="refresh" size="sm" />
              {{ t('common.tryAgain') }}
            </button>
          </div>
          <template v-else-if="selectedMultiplierHistory">
            <div class="border-b border-gray-100 pb-3 dark:border-dark-700">
              <div class="flex flex-wrap items-center gap-2">
                <h4 class="break-words text-sm font-semibold text-gray-900 dark:text-gray-100">{{ selectedMultiplierHistory.name }}</h4>
                <span class="rounded bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ selectedMultiplierHistory.platform || '—' }}</span>
                <span class="text-sm font-medium text-primary-600 dark:text-primary-400">{{ formatMultiplier(selectedMultiplierHistory.current_multiplier) }}</span>
              </div>
              <p v-if="selectedMultiplierHistory.description" class="mt-2 break-words text-sm text-gray-500 dark:text-gray-400">{{ selectedMultiplierHistory.description }}</p>
            </div>
            <div v-if="hasMultiplierChartPoints" class="h-72" data-test="multiplier-history-chart">
              <Line :data="multiplierChartData" :options="multiplierChartOptions" />
            </div>
            <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noMultiplierHistory') }}</p>
          </template>
          <p v-else class="py-16 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noMultiplierHistory') }}</p>
        </template>
      </div>
    </BaseDialog>

    <BaseDialog :show="Boolean(deleteTarget)" :title="t('admin.customFeatures.upstream.deleteTitle')" width="narrow" @close="deleteTarget = null">
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('admin.customFeatures.upstream.deleteMessage', { name: deleteTarget?.name || '' }) }}</p>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="deleteTarget = null">{{ t('admin.customFeatures.upstream.cancel') }}</button>
        <button type="button" class="btn btn-danger" :disabled="deleting" @click="confirmDelete">{{ t('admin.customFeatures.upstream.delete') }}</button>
      </template>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  CategoryScale,
  Chart as ChartJS,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip,
  type TooltipItem
} from 'chart.js'
import { Line } from 'vue-chartjs'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import upstreamsAPI, {
  type UpstreamDailyStat,
  type UpstreamGroup,
  type UpstreamGroupMultiplierHistory,
  type UpstreamPlatform,
  type UpstreamSite,
  type UpstreamWritePayload
} from '@/api/admin/upstreams'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const { t } = useI18n()
const appStore = useAppStore()
const sites = ref<UpstreamSite[]>([])
const loading = ref(true)
const loadError = ref('')
const page = ref(1)
const pageSize = 20
const total = ref(0)
const pages = ref(1)
const search = ref('')
const platformFilter = ref<UpstreamPlatform | ''>('')
const enabledFilter = ref('')
const syncingAll = ref(false)
const formOpen = ref(false)
const editingSite = ref<UpstreamSite | null>(null)
const saving = ref(false)
const deleteTarget = ref<UpstreamSite | null>(null)
const deleting = ref(false)
const expandedSiteIDs = ref<Set<number>>(new Set())
interface SiteGroupState {
  groups: UpstreamGroup[]
  loaded: boolean
  loading: boolean
  error: string
  syncedAt: string | null
  requestedSyncAt: string | null
}
const siteGroupStates = reactive<Record<number, SiteGroupState>>({})
const groupRequestVersions = new Map<number, number>()
const detailSite = ref<UpstreamSite | null>(null)
const detailTab = ref<'groups' | 'usage' | 'multiplier'>('groups')
const detailTabs = ['groups', 'usage', 'multiplier'] as const
const detailGroups = ref<UpstreamGroup[]>([])
const detailHistory = ref<UpstreamDailyStat[]>([])
const detailLoading = ref(false)
const historyDays = ref<7 | 30 | 90>(30)
const historyRanges = [7, 30, 90] as const
const multiplierHistories = ref<UpstreamGroupMultiplierHistory[]>([])
const selectedMultiplierRemoteID = ref('')
const multiplierDays = ref<7 | 30 | 90>(30)
const multiplierLoading = ref(false)
const multiplierError = ref('')
const multiplierLoadedSiteID = ref<number | null>(null)
const multiplierLoadedDays = ref<7 | 30 | 90 | null>(null)
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let disposed = false
let multiplierRequestVersion = 0
let detailRequestVersion = 0
let historyRequestVersion = 0
let siteListRequestVersion = 0

const form = reactive<UpstreamWritePayload>({
  name: '', base_url: '', platform: 'sub2api', auth_mode: 'password', account: '',
  password: '', access_token: '', refresh_token: '', enabled: true
})

const columns = computed<Column[]>(() => [
  { key: 'site', label: t('admin.customFeatures.upstream.columns.site'), class: 'min-w-56' },
  { key: 'platform', label: t('admin.customFeatures.upstream.platform') },
  { key: 'status', label: t('admin.customFeatures.upstream.columns.status') },
  { key: 'balance_usd', label: t('admin.customFeatures.upstream.columns.balance') },
  { key: 'today', label: t('admin.customFeatures.upstream.columns.today') },
  { key: 'total', label: t('admin.customFeatures.upstream.columns.total') },
  { key: 'last_synced_at', label: t('admin.customFeatures.upstream.columns.lastSync') },
  { key: 'actions', label: t('admin.customFeatures.upstream.columns.actions'), class: 'min-w-56' }
])

const expandedSiteKeys = computed(() => Array.from(expandedSiteIDs.value))

const groupColumns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.customFeatures.upstream.groupColumns.name') },
  { key: 'platform', label: t('admin.customFeatures.upstream.platform') },
  { key: 'multiplier', label: t('admin.customFeatures.upstream.groupColumns.multiplier') },
  { key: 'today_tokens', label: t('admin.customFeatures.upstream.groupColumns.tokens') },
  { key: 'today_cost_usd', label: t('admin.customFeatures.upstream.groupColumns.cost') }
])

const historyChartData = computed(() => ({
  labels: detailHistory.value.map((item) => item.date.slice(0, 10)),
  datasets: [
    { label: t('admin.customFeatures.upstream.chart.tokens'), data: detailHistory.value.map((item) => item.tokens), borderColor: '#2563eb', backgroundColor: '#2563eb20', tension: 0.25, yAxisID: 'tokens' },
    { label: t('admin.customFeatures.upstream.chart.cost'), data: detailHistory.value.map((item) => item.cost_usd), borderColor: '#dc2626', backgroundColor: '#dc262620', tension: 0.25, yAxisID: 'currency' },
    { label: t('admin.customFeatures.upstream.chart.balance'), data: detailHistory.value.map((item) => item.balance_usd), borderColor: '#059669', backgroundColor: '#05966920', tension: 0.25, yAxisID: 'currency' }
  ]
}))

const historyChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    tokens: {
      type: 'linear' as const,
      position: 'left' as const,
      beginAtZero: true,
      ticks: { callback: (value: string | number) => formatCompactNumber(Number(value)) }
    },
    currency: { type: 'linear' as const, position: 'right' as const, beginAtZero: true, grid: { drawOnChartArea: false } }
  },
  plugins: { legend: { position: 'top' as const } }
}))

const selectedMultiplierHistory = computed(() => (
  multiplierHistories.value.find((item) => item.remote_id === selectedMultiplierRemoteID.value) || null
))
const hasMultiplierChartPoints = computed(() => (
  selectedMultiplierHistory.value?.points.some((point) => point.multiplier != null) === true
))

const multiplierChartData = computed(() => {
  const points = selectedMultiplierHistory.value?.points || []
  return {
    datasets: [{
      label: t('admin.customFeatures.upstream.chart.multiplier'),
      data: points.map((point) => ({ x: new Date(point.recorded_at).getTime(), y: point.multiplier })),
      borderColor: '#2563eb',
      backgroundColor: '#2563eb20',
      stepped: 'after' as const,
      tension: 0,
      spanGaps: false,
      pointRadius: 3
    }]
  }
})

const multiplierChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    y: {
      type: 'linear' as const,
      beginAtZero: false,
      ticks: { callback: (value: string | number) => `${value}×` }
    },
    x: {
      type: 'linear' as const,
      ticks: {
        maxRotation: 0,
        callback: (value: string | number) => formatShortDateTime(new Date(Number(value)).toISOString())
      }
    }
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        title: (items: TooltipItem<'line'>[]) => {
          const timestamp = items[0]?.parsed.x
          return timestamp == null ? '' : formatDateTime(new Date(timestamp).toISOString())
        },
        label: (item: TooltipItem<'line'>) => `${t('admin.customFeatures.upstream.chart.multiplier')}: ${item.parsed.y == null ? '—' : formatMultiplier(item.parsed.y)}`
      }
    }
  }
}))

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  if (disposed) return
  const busy = sites.value.some((site) => site.status === 'pending' || site.status === 'syncing')
  refreshTimer = setTimeout(() => loadSites(true), busy ? 2000 : 30000)
}

function groupState(siteID: number): SiteGroupState {
  if (!siteGroupStates[siteID]) {
    siteGroupStates[siteID] = { groups: [], loaded: false, loading: false, error: '', syncedAt: null, requestedSyncAt: null }
  }
  return siteGroupStates[siteID]
}

function isSiteExpanded(siteID: number) {
  return expandedSiteIDs.value.has(siteID)
}

function toggleSiteGroups(site: UpstreamSite) {
  const next = new Set(expandedSiteIDs.value)
  if (next.has(site.id)) {
    next.delete(site.id)
  } else {
    next.add(site.id)
    void loadSiteGroups(site)
  }
  expandedSiteIDs.value = next
}

async function loadSiteGroups(site: UpstreamSite, force = false) {
  const state = groupState(site.id)
  const syncVersion = site.last_synced_at
  if (!force && state.loaded && state.syncedAt === syncVersion && !state.error) return
  if (!force && state.loading && state.requestedSyncAt === syncVersion) return

  const requestVersion = (groupRequestVersions.get(site.id) || 0) + 1
  groupRequestVersions.set(site.id, requestVersion)
  state.loading = true
  state.error = ''
  state.requestedSyncAt = syncVersion
  try {
    const groups = await upstreamsAPI.groups(site.id)
    if (groupRequestVersions.get(site.id) !== requestVersion) return
    state.groups = groups || []
    state.loaded = true
    state.syncedAt = syncVersion
  } catch (error) {
    if (groupRequestVersions.get(site.id) !== requestVersion) return
    state.error = extractApiErrorMessage(error, t('admin.customFeatures.upstream.groupsLoadFailed'))
  } finally {
    if (groupRequestVersions.get(site.id) === requestVersion) state.loading = false
  }
}

async function loadSites(silent = false) {
  const requestVersion = ++siteListRequestVersion
  if (!silent) loading.value = true
  try {
    const result = await upstreamsAPI.list({
      page: page.value, page_size: pageSize, search: search.value.trim(), platform: platformFilter.value,
      enabled: enabledFilter.value === '' ? undefined : enabledFilter.value === 'true'
    })
    if (requestVersion !== siteListRequestVersion || disposed) return
    sites.value = result.items || []
    total.value = result.total
    pages.value = result.pages || 1
    loadError.value = ''
    for (const site of sites.value) {
      const state = groupState(site.id)
      if (isSiteExpanded(site.id) && (!state.loaded || state.syncedAt !== site.last_synced_at)) {
        void loadSiteGroups(site)
      }
    }
  } catch (error) {
    if (requestVersion !== siteListRequestVersion || disposed) return
    if (!silent) loadError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.loadFailed'))
  } finally {
    if (requestVersion === siteListRequestVersion) {
      loading.value = false
      scheduleRefresh()
    }
  }
}

function applyFilters() { page.value = 1; loadSites(false) }
function changePage(nextPage: number) { page.value = nextPage; loadSites(false) }

function resetForm() {
  Object.assign(form, { name: '', base_url: '', platform: 'sub2api', auth_mode: 'password', account: '', password: '', access_token: '', refresh_token: '', enabled: true })
}

function openCreate() { editingSite.value = null; resetForm(); formOpen.value = true }
function openEdit(site: UpstreamSite) {
  editingSite.value = site
  Object.assign(form, { name: site.name, base_url: site.base_url, platform: site.platform, auth_mode: site.auth_mode, account: site.account, password: '', access_token: '', refresh_token: '', enabled: site.enabled })
  formOpen.value = true
}
function closeForm() { if (!saving.value) formOpen.value = false }
function handlePlatformChange() { if (form.platform === 'newapi') form.auth_mode = 'password' }

async function submitForm() {
  if (form.auth_mode === 'token' && !editingSite.value && !form.access_token?.trim() && !form.refresh_token?.trim()) {
    appStore.showError(t('admin.customFeatures.upstream.tokenRequired'))
    return
  }
  saving.value = true
  try {
    const payload: UpstreamWritePayload = { ...form, name: form.name.trim(), base_url: form.base_url.trim(), account: form.account.trim(), password: form.password?.trim(), access_token: form.access_token?.trim(), refresh_token: form.refresh_token?.trim() }
    if (editingSite.value) await upstreamsAPI.update(editingSite.value.id, payload)
    else await upstreamsAPI.create(payload)
    appStore.showSuccess(t('admin.customFeatures.upstream.saved'))
    formOpen.value = false
    await loadSites(true)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.saveFailed')))
  } finally { saving.value = false }
}

async function syncSite(site: UpstreamSite) {
  try { await upstreamsAPI.sync(site.id); site.status = 'pending'; scheduleRefresh() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.syncFailed'))) }
}

async function syncAllSites() {
  syncingAll.value = true
  try { await upstreamsAPI.syncAll(); sites.value.forEach((site) => { site.status = 'pending' }); scheduleRefresh() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.syncFailed'))) }
  finally { syncingAll.value = false }
}

async function toggleSite(site: UpstreamSite) {
  try { const updated = await upstreamsAPI.setEnabled(site.id, !site.enabled); Object.assign(site, updated); scheduleRefresh() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.saveFailed'))) }
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  const deletedID = deleteTarget.value.id
  deleting.value = true
  try {
    await upstreamsAPI.remove(deletedID)
    deleteTarget.value = null
    const next = new Set(expandedSiteIDs.value)
    next.delete(deletedID)
    expandedSiteIDs.value = next
    delete siteGroupStates[deletedID]
    groupRequestVersions.delete(deletedID)
    await loadSites(true)
    appStore.showSuccess(t('admin.customFeatures.upstream.deleted'))
  }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.deleteFailed'))) }
  finally { deleting.value = false }
}

async function openDetails(site: UpstreamSite) {
  const requestVersion = ++detailRequestVersion
  historyRequestVersion++
  detailSite.value = site
  detailTab.value = 'groups'
  detailLoading.value = true
  detailGroups.value = []
  detailHistory.value = []
  multiplierHistories.value = []
  selectedMultiplierRemoteID.value = ''
  multiplierError.value = ''
  multiplierLoadedSiteID.value = null
  multiplierLoadedDays.value = null
  multiplierRequestVersion++
  try {
    const [groups, history] = await Promise.all([upstreamsAPI.groups(site.id), upstreamsAPI.history(site.id, historyDays.value)])
    if (requestVersion !== detailRequestVersion || detailSite.value?.id !== site.id) return
    detailGroups.value = groups || []
    detailHistory.value = history || []
    const state = groupState(site.id)
    state.groups = detailGroups.value
    state.loaded = true
    state.syncedAt = site.last_synced_at
    state.error = ''
  }
  catch (error) {
    if (requestVersion === detailRequestVersion) {
      appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed')))
    }
  }
  finally {
    if (requestVersion === detailRequestVersion) detailLoading.value = false
  }
}
function closeDetails() {
  detailRequestVersion++
  historyRequestVersion++
  multiplierRequestVersion++
  detailSite.value = null
  detailGroups.value = []
  detailHistory.value = []
  multiplierHistories.value = []
  selectedMultiplierRemoteID.value = ''
  multiplierError.value = ''
  multiplierLoadedSiteID.value = null
  multiplierLoadedDays.value = null
  multiplierLoading.value = false
  detailLoading.value = false
}

function selectDetailTab(tab: typeof detailTabs[number]) {
  detailTab.value = tab
  if (tab === 'multiplier') void loadMultiplierHistory()
}

async function changeHistoryRange(days: 7 | 30 | 90) {
  if (!detailSite.value || days === historyDays.value) return
  const siteID = detailSite.value.id
  const requestVersion = ++historyRequestVersion
  historyDays.value = days
  detailLoading.value = true
  try {
    const history = await upstreamsAPI.history(siteID, days)
    if (requestVersion === historyRequestVersion && detailSite.value?.id === siteID) detailHistory.value = history
  }
  catch (error) {
    if (requestVersion === historyRequestVersion) {
      appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed')))
    }
  }
  finally {
    if (requestVersion === historyRequestVersion) detailLoading.value = false
  }
}

async function loadMultiplierHistory(force = false) {
  const site = detailSite.value
  if (!site) return
  if (!force && multiplierLoadedSiteID.value === site.id && multiplierLoadedDays.value === multiplierDays.value) return
  const requestVersion = ++multiplierRequestVersion
  multiplierLoading.value = true
  multiplierError.value = ''
  try {
    const histories = await upstreamsAPI.multiplierHistory(site.id, multiplierDays.value)
    if (requestVersion !== multiplierRequestVersion || detailSite.value?.id !== site.id) return
    multiplierHistories.value = histories || []
    if (!multiplierHistories.value.some((item) => item.remote_id === selectedMultiplierRemoteID.value)) {
      selectedMultiplierRemoteID.value = multiplierHistories.value[0]?.remote_id || ''
    }
    multiplierLoadedSiteID.value = site.id
    multiplierLoadedDays.value = multiplierDays.value
  } catch (error) {
    if (requestVersion !== multiplierRequestVersion) return
    multiplierError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.multiplierHistoryFailed'))
  } finally {
    if (requestVersion === multiplierRequestVersion) multiplierLoading.value = false
  }
}

function changeMultiplierRange(days: 7 | 30 | 90) {
  if (days === multiplierDays.value) return
  multiplierDays.value = days
  void loadMultiplierHistory(true)
}

function platformLabel(platform: UpstreamPlatform) { return platform === 'newapi' ? 'New API' : 'Sub2API' }
function formatMoney(value: number | null | undefined) { return value == null ? '—' : `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 })}` }
function formatTokens(value: number) { return formatCompactNumber(Number(value || 0)) }
function formatExactTokens(value: number) { return Number(value || 0).toLocaleString('en-US') }
function formatMultiplier(value: number | null | undefined) {
  return value == null ? '—' : `${value.toLocaleString(undefined, { maximumFractionDigits: 6 })}×`
}
function formatDateTime(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
function formatShortDateTime(value: string | null | undefined) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hour = String(date.getHours()).padStart(2, '0')
  const minute = String(date.getMinutes()).padStart(2, '0')
  return `${month}-${day} ${hour}:${minute}`
}
function statusClass(status: UpstreamSite['status']) {
  return {
    pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
    syncing: 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-300',
    healthy: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
    error: 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  }[status]
}

onMounted(() => loadSites(false))
onBeforeUnmount(() => { disposed = true; if (refreshTimer) clearTimeout(refreshTimer) })
</script>

<style scoped>
.icon-action {
  @apply inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-100;
}
</style>
