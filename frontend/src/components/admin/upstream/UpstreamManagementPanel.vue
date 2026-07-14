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
    >
      <template #cell-site="{ row }">
        <div class="max-w-56 whitespace-normal">
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
          <p class="font-medium text-gray-800 dark:text-gray-200">{{ formatTokens(row.today_tokens) }}</p>
          <p class="text-gray-500 dark:text-gray-400">{{ formatMoney(row.today_cost_usd) }}</p>
        </div>
      </template>
      <template #cell-total="{ row }">
        <div class="space-y-1 text-xs">
          <p class="font-medium text-gray-800 dark:text-gray-200">{{ formatTokens(row.total_tokens) }}</p>
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
        <div class="flex border-b border-gray-200 dark:border-dark-700">
          <button v-for="tab in detailTabs" :key="tab" type="button" class="border-b-2 px-4 py-2 text-sm font-medium" :class="detailTab === tab ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" @click="detailTab = tab">
            {{ t(`admin.customFeatures.upstream.detailTabs.${tab}`) }}
          </button>
        </div>
        <div v-if="detailLoading" class="flex h-56 items-center justify-center">
          <span class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></span>
        </div>
        <template v-else-if="detailTab === 'groups'">
          <DataTable :columns="groupColumns" :data="detailGroups" row-key="remote_id" :sticky-actions-column="false" :expandable-actions="false">
            <template #cell-multiplier="{ row }">{{ row.multiplier == null ? '—' : `${row.multiplier}×` }}</template>
            <template #cell-today_tokens="{ row }">{{ formatTokens(row.today_tokens) }}</template>
            <template #cell-today_cost_usd="{ row }">{{ formatMoney(row.today_cost_usd) }}</template>
            <template #empty><p class="py-8 text-center text-sm text-gray-500">{{ t('admin.customFeatures.upstream.noGroups') }}</p></template>
          </DataTable>
        </template>
        <template v-else>
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
  Tooltip
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
  type UpstreamPlatform,
  type UpstreamSite,
  type UpstreamWritePayload
} from '@/api/admin/upstreams'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

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
const detailSite = ref<UpstreamSite | null>(null)
const detailTab = ref<'groups' | 'history'>('groups')
const detailTabs = ['groups', 'history'] as const
const detailGroups = ref<UpstreamGroup[]>([])
const detailHistory = ref<UpstreamDailyStat[]>([])
const detailLoading = ref(false)
const historyDays = ref<7 | 30 | 90>(30)
const historyRanges = [7, 30, 90] as const
let refreshTimer: ReturnType<typeof setTimeout> | null = null
let disposed = false

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
    tokens: { type: 'linear' as const, position: 'left' as const, beginAtZero: true },
    currency: { type: 'linear' as const, position: 'right' as const, beginAtZero: true, grid: { drawOnChartArea: false } }
  },
  plugins: { legend: { position: 'top' as const } }
}))

function scheduleRefresh() {
  if (refreshTimer) clearTimeout(refreshTimer)
  if (disposed) return
  const busy = sites.value.some((site) => site.status === 'pending' || site.status === 'syncing')
  refreshTimer = setTimeout(() => loadSites(true), busy ? 2000 : 30000)
}

async function loadSites(silent = false) {
  if (!silent) loading.value = true
  try {
    const result = await upstreamsAPI.list({
      page: page.value, page_size: pageSize, search: search.value.trim(), platform: platformFilter.value,
      enabled: enabledFilter.value === '' ? undefined : enabledFilter.value === 'true'
    })
    sites.value = result.items || []
    total.value = result.total
    pages.value = result.pages || 1
    loadError.value = ''
  } catch (error) {
    if (!silent) loadError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.loadFailed'))
  } finally {
    if (!silent) loading.value = false
    scheduleRefresh()
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
  deleting.value = true
  try { await upstreamsAPI.remove(deleteTarget.value.id); deleteTarget.value = null; await loadSites(true); appStore.showSuccess(t('admin.customFeatures.upstream.deleted')) }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.deleteFailed'))) }
  finally { deleting.value = false }
}

async function openDetails(site: UpstreamSite) {
  detailSite.value = site; detailTab.value = 'groups'; detailLoading.value = true
  try { [detailGroups.value, detailHistory.value] = await Promise.all([upstreamsAPI.groups(site.id), upstreamsAPI.history(site.id, historyDays.value)]) }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed'))) }
  finally { detailLoading.value = false }
}
function closeDetails() { detailSite.value = null; detailGroups.value = []; detailHistory.value = [] }
async function changeHistoryRange(days: 7 | 30 | 90) {
  if (!detailSite.value || days === historyDays.value) return
  historyDays.value = days; detailLoading.value = true
  try { detailHistory.value = await upstreamsAPI.history(detailSite.value.id, days) }
  catch (error) { appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.detailFailed'))) }
  finally { detailLoading.value = false }
}

function platformLabel(platform: UpstreamPlatform) { return platform === 'newapi' ? 'New API' : 'Sub2API' }
function formatMoney(value: number | null | undefined) { return value == null ? '—' : `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 4 })}` }
function formatTokens(value: number) { return Number(value || 0).toLocaleString() }
function formatDateTime(value: string | null) { return value ? new Date(value).toLocaleString() : '—' }
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
