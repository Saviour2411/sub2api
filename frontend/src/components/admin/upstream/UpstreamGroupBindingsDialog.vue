<template>
  <BaseDialog
    :show="show"
    :title="t('admin.customFeatures.upstream.bindings.title', { name: group?.name || '' })"
    width="extra-wide"
    @close="closeDialog"
  >
    <div class="space-y-5" data-test="upstream-bindings-dialog">
      <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-gray-600 dark:text-gray-300">
        <span>
          {{ t('admin.customFeatures.upstream.bindings.multiplier') }}:
          <strong class="text-gray-900 dark:text-gray-100">{{ formatMultiplier(group?.multiplier) }}</strong>
        </span>
        <span>
          {{ t('admin.customFeatures.upstream.bindings.boundCount', { count: draftBindings.length }) }}
        </span>
      </div>

      <div class="flex items-start gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2.5 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-200">
        <Icon name="exclamationTriangle" size="sm" class="mt-0.5 flex-shrink-0" />
        <p>{{ t('admin.customFeatures.upstream.bindings.globalPriorityWarning') }}</p>
      </div>

      <div
        v-if="!canAddBindings"
        class="flex items-start gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2.5 text-sm text-blue-800 dark:border-blue-900 dark:bg-blue-950/20 dark:text-blue-200"
        data-test="upstream-bindings-frozen"
      >
        <Icon name="infoCircle" size="sm" class="mt-0.5 flex-shrink-0" />
        <p>{{ t('admin.customFeatures.upstream.bindings.frozenHint') }}</p>
      </div>

      <section class="space-y-2">
        <h4 class="text-sm font-semibold text-gray-900 dark:text-gray-100">
          {{ t('admin.customFeatures.upstream.bindings.currentBindings') }}
        </h4>
        <DataTable :columns="bindingColumns" :data="draftBindings" row-key="account_id" compact>
          <template #cell-account="{ row }">
            <div class="max-w-56 whitespace-normal">
              <p class="font-medium">{{ row.account_name }}</p>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ row.account_platform }}</p>
            </div>
          </template>
          <template #cell-local_group_name="{ row }">
            <span class="whitespace-normal">{{ row.local_group_name }}</span>
          </template>
          <template #cell-account_status="{ row }">
            <span class="rounded px-2 py-1 text-xs font-medium" :class="accountStatusClass(row.account_status)">
              {{ accountStatusLabel(row.account_status) }}
            </span>
          </template>
          <template #cell-account_priority="{ row }">{{ row.account_priority }}</template>
          <template #cell-actions="{ row }">
            <button
              type="button"
              class="icon-action text-red-600 hover:text-red-700 dark:text-red-400"
              :title="t('admin.customFeatures.upstream.bindings.remove')"
              :data-test="`remove-upstream-binding-${row.account_id}`"
              @click="removeBinding(row.account_id)"
            >
              <Icon name="trash" size="sm" />
            </button>
          </template>
          <template #empty>
            <p class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.customFeatures.upstream.bindings.noBindings') }}
            </p>
          </template>
        </DataTable>
      </section>

      <section v-if="canAddBindings" class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-[minmax(180px,260px)_minmax(240px,1fr)_auto] md:items-end">
          <div>
            <label for="upstream-binding-local-group" class="input-label">
              {{ t('admin.customFeatures.upstream.bindings.localGroup') }}
            </label>
            <select
              id="upstream-binding-local-group"
              v-model.number="selectedGroupID"
              class="input"
              :disabled="groupsLoading || activeGroups.length === 0"
              @change="handleGroupChange"
            >
              <option v-for="item in activeGroups" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </div>
          <div>
            <label for="upstream-binding-account-search" class="input-label">
              {{ t('admin.customFeatures.upstream.bindings.searchAccount') }}
            </label>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-3 text-gray-400" />
              <input
                id="upstream-binding-account-search"
                v-model="accountSearch"
                type="search"
                class="input pl-9"
                :placeholder="t('admin.customFeatures.upstream.bindings.searchPlaceholder')"
                :disabled="!selectedGroupID"
                @keyup.enter="applyAccountSearch"
              />
            </div>
          </div>
          <button type="button" class="btn btn-secondary inline-flex items-center justify-center gap-2" :disabled="!selectedGroupID" @click="applyAccountSearch">
            <Icon name="search" size="sm" />
            {{ t('admin.customFeatures.upstream.bindings.search') }}
          </button>
        </div>

        <div v-if="groupsError" class="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-950/20 dark:text-red-300">
          {{ groupsError }}
        </div>
        <p v-else-if="!groupsLoading && activeGroups.length === 0" class="py-5 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.customFeatures.upstream.bindings.noActiveGroups') }}
        </p>
        <template v-else>
          <DataTable :columns="accountColumns" :data="accounts" :loading="accountsLoading || groupsLoading" row-key="id" compact>
            <template #cell-account="{ row }">
              <div class="max-w-64 whitespace-normal">
                <p class="font-medium">{{ row.name }}</p>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ row.platform }}</p>
              </div>
            </template>
            <template #cell-status="{ row }">
              <span class="rounded px-2 py-1 text-xs font-medium" :class="accountStatusClass(row.status)">
                {{ accountStatusLabel(row.status) }}
              </span>
            </template>
            <template #cell-priority="{ row }">{{ row.priority }}</template>
            <template #cell-actions="{ row }">
              <button
                type="button"
                class="btn btn-secondary inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs"
                :disabled="isAccountSelected(row.id)"
                :data-test="`add-upstream-binding-${row.id}`"
                @click="addBinding(row)"
              >
                <Icon :name="isAccountSelected(row.id) ? 'check' : 'plus'" size="xs" />
                {{ isAccountSelected(row.id) ? t('admin.customFeatures.upstream.bindings.selected') : t('admin.customFeatures.upstream.bindings.addAccount') }}
              </button>
            </template>
            <template #empty>
              <p class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                {{ accountsError || t('admin.customFeatures.upstream.bindings.noAccounts') }}
              </p>
            </template>
          </DataTable>
          <Pagination
            v-if="accountTotal > accountPageSize"
            :total="accountTotal"
            :page="accountPage"
            :page-size="accountPageSize"
            :show-page-size-selector="false"
            @update:page="changeAccountPage"
          />
        </template>
      </section>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="closeDialog">
          {{ t('admin.customFeatures.upstream.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary inline-flex items-center gap-2"
          :disabled="saving || !hasChanges"
          data-test="save-upstream-bindings"
          @click="saveBindings"
        >
          <span v-if="saving" class="h-4 w-4 animate-spin rounded-full border-b-2 border-white"></span>
          <Icon v-else name="check" size="sm" />
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import groupsAPI from '@/api/admin/groups'
import accountsAPI from '@/api/admin/accounts'
import upstreamsAPI, {
  type UpstreamGroup,
  type UpstreamGroupAccountBinding,
  type UpstreamSite
} from '@/api/admin/upstreams'
import type { Account, AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

interface DraftBinding {
  local_group_id: number
  local_group_name: string
  account_id: number
  account_name: string
  account_platform: string
  account_status: string
  account_priority: number
}

const props = defineProps<{
  show: boolean
  site: UpstreamSite | null
  group: UpstreamGroup | null
}>()

const emit = defineEmits<{
  close: []
  saved: [group: UpstreamGroup]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const activeGroups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)
const groupsError = ref('')
const selectedGroupID = ref<number | null>(null)
const draftBindings = ref<DraftBinding[]>([])
const accounts = ref<Account[]>([])
const accountsLoading = ref(false)
const accountsError = ref('')
const accountSearch = ref('')
const accountPage = ref(1)
const accountPageSize = 10
const accountTotal = ref(0)
const saving = ref(false)
let dialogVersion = 0
let accountsRequestVersion = 0

const canAddBindings = computed(() => props.group?.available === true && props.group.multiplier != null)
const selectedGroup = computed(() => activeGroups.value.find((item) => item.id === selectedGroupID.value) || null)
const originalBindingKeys = computed(() => normalizedBindingKeys(props.group?.bindings || []))
const currentBindingKeys = computed(() => normalizedBindingKeys(draftBindings.value))
const hasChanges = computed(() => originalBindingKeys.value !== currentBindingKeys.value)

const bindingColumns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.customFeatures.upstream.bindings.account') },
  { key: 'local_group_name', label: t('admin.customFeatures.upstream.bindings.localGroup') },
  { key: 'account_status', label: t('admin.customFeatures.upstream.bindings.status') },
  { key: 'account_priority', label: t('admin.customFeatures.upstream.bindings.priority') },
  { key: 'actions', label: t('admin.customFeatures.upstream.columns.actions'), class: 'w-20' }
])

const accountColumns = computed<Column[]>(() => [
  { key: 'account', label: t('admin.customFeatures.upstream.bindings.account') },
  { key: 'status', label: t('admin.customFeatures.upstream.bindings.status') },
  { key: 'priority', label: t('admin.customFeatures.upstream.bindings.priority') },
  { key: 'actions', label: t('admin.customFeatures.upstream.columns.actions'), class: 'w-28' }
])

watch(
  [() => props.show, () => props.group?.id],
  ([show]) => {
    if (show) void initializeDialog()
    else invalidateRequests()
  }
)

function normalizedBindingKeys(bindings: Array<Pick<DraftBinding, 'local_group_id' | 'account_id'> | UpstreamGroupAccountBinding>) {
  return bindings
    .map((binding) => `${binding.local_group_id}:${binding.account_id}`)
    .sort()
    .join('|')
}

function mapBinding(binding: UpstreamGroupAccountBinding): DraftBinding {
  return {
    local_group_id: binding.local_group_id,
    local_group_name: binding.local_group_name,
    account_id: binding.account_id,
    account_name: binding.account_name,
    account_platform: binding.account_platform,
    account_status: binding.account_status,
    account_priority: binding.account_priority
  }
}

function invalidateRequests() {
  dialogVersion++
  accountsRequestVersion++
}

async function initializeDialog() {
  const version = ++dialogVersion
  accountsRequestVersion++
  draftBindings.value = (props.group?.bindings || []).map(mapBinding)
  activeGroups.value = []
  selectedGroupID.value = null
  accounts.value = []
  accountTotal.value = 0
  accountPage.value = 1
  accountSearch.value = ''
  groupsError.value = ''
  accountsError.value = ''
  groupsLoading.value = false
  accountsLoading.value = false
  if (!canAddBindings.value) return

  groupsLoading.value = true
  let shouldLoadAccounts = false
  try {
    const groups = await groupsAPI.getAll()
    if (version !== dialogVersion || !props.show) return
    activeGroups.value = (groups || []).filter((item) => item.status === 'active')
    const preferredGroupID = draftBindings.value.find((binding) => activeGroups.value.some((item) => item.id === binding.local_group_id))?.local_group_id
    selectedGroupID.value = preferredGroupID || activeGroups.value[0]?.id || null
    shouldLoadAccounts = Boolean(selectedGroupID.value)
  } catch (error) {
    if (version === dialogVersion) {
      groupsError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.bindings.groupsLoadFailed'))
    }
  } finally {
    if (version === dialogVersion) groupsLoading.value = false
  }
  if (shouldLoadAccounts && version === dialogVersion && props.show) await loadAccounts()
}

async function loadAccounts() {
  if (!selectedGroupID.value || !canAddBindings.value) {
    accounts.value = []
    accountTotal.value = 0
    return
  }
  const version = ++accountsRequestVersion
  accountsLoading.value = true
  accountsError.value = ''
  try {
    const result = await accountsAPI.list(accountPage.value, accountPageSize, {
      group: String(selectedGroupID.value),
      search: accountSearch.value.trim()
    })
    if (version !== accountsRequestVersion || !props.show) return
    accounts.value = result.items || []
    accountTotal.value = result.total || 0
  } catch (error) {
    if (version !== accountsRequestVersion) return
    accounts.value = []
    accountTotal.value = 0
    accountsError.value = extractApiErrorMessage(error, t('admin.customFeatures.upstream.bindings.accountsLoadFailed'))
  } finally {
    if (version === accountsRequestVersion) accountsLoading.value = false
  }
}

function handleGroupChange() {
  accountPage.value = 1
  accountSearch.value = ''
  void loadAccounts()
}

function applyAccountSearch() {
  accountPage.value = 1
  void loadAccounts()
}

function changeAccountPage(nextPage: number) {
  accountPage.value = nextPage
  void loadAccounts()
}

function isAccountSelected(accountID: number) {
  return draftBindings.value.some((binding) => binding.account_id === accountID)
}

function addBinding(account: Account) {
  const group = selectedGroup.value
  if (!group || isAccountSelected(account.id)) return
  draftBindings.value.push({
    local_group_id: group.id,
    local_group_name: group.name,
    account_id: account.id,
    account_name: account.name,
    account_platform: account.platform,
    account_status: account.status,
    account_priority: account.priority
  })
}

function removeBinding(accountID: number) {
  draftBindings.value = draftBindings.value.filter((binding) => binding.account_id !== accountID)
}

async function saveBindings() {
  if (!props.site || !props.group || saving.value || !hasChanges.value) return
  saving.value = true
  try {
    const updated = await upstreamsAPI.replaceGroupBindings(
      props.site.id,
      props.group.id,
      draftBindings.value.map((binding) => ({
        local_group_id: binding.local_group_id,
        account_id: binding.account_id
      }))
    )
    appStore.showSuccess(t('admin.customFeatures.upstream.bindings.saved'))
    emit('saved', updated)
  } catch (error) {
    if (extractApiErrorCode(error) === 'UPSTREAM_BINDING_CONFLICT') {
      appStore.showError(t('admin.customFeatures.upstream.bindings.conflict'))
    } else {
      appStore.showError(extractApiErrorMessage(error, t('admin.customFeatures.upstream.bindings.saveFailed')))
    }
  } finally {
    saving.value = false
  }
}

function closeDialog() {
  if (!saving.value) emit('close')
}

function formatMultiplier(value: number | null | undefined) {
  return value == null ? '—' : `${value.toLocaleString(undefined, { maximumFractionDigits: 6 })}×`
}

function accountStatusLabel(status: string) {
  const key = `admin.accounts.status.${status}`
  const label = t(key)
  return label === key ? status : label
}

function accountStatusClass(status: string) {
  if (status === 'active') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
  if (status === 'error') return 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
}
</script>

<style scoped>
.icon-action {
  @apply inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-gray-100;
}
</style>
