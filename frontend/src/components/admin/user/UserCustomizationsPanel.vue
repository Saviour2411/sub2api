<template>
  <div data-test="user-customizations-panel" class="space-y-6">
    <section class="border-b border-gray-200 pb-6 dark:border-dark-700">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('userCustomization.paymentMethods') }}</h2>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary inline-flex items-center gap-2" :disabled="methodsLoading || methods.length >= 20" @click="methods.push({ key: '', value: '' })"><Icon name="plus" size="sm" />{{ t('common.add') }}</button>
          <button type="button" class="btn btn-primary inline-flex items-center gap-2" :disabled="methodsLoading || methodsSaving || methodsFailed" @click="saveMethods"><Icon name="check" size="sm" />{{ t('common.save') }}</button>
        </div>
      </div>
      <div v-if="methodsFailed" role="alert" class="text-sm text-red-600">
        {{ t('userCustomization.loadFailed') }}
        <button type="button" class="ml-2 underline" @click="loadMethods">{{ t('common.retry') }}</button>
      </div>
      <p v-else-if="methodsLoading" class="text-sm text-gray-500">{{ t('common.loading') }}</p>
      <p v-else-if="!methods.length" class="text-sm text-gray-500">{{ t('userCustomization.noPaymentMethods') }}</p>
      <fieldset v-else :disabled="methodsSaving" class="space-y-2">
        <div v-for="(method, index) in methods" :key="index" class="grid grid-cols-[minmax(0,1fr)_auto] gap-2 sm:grid-cols-[minmax(120px,1fr)_minmax(0,3fr)_auto]">
          <input v-model="method.key" maxlength="50" class="input min-w-0" :aria-label="t('userCustomization.methodKey')" :placeholder="t('userCustomization.methodKey')" />
          <input v-model="method.value" maxlength="500" class="input col-span-2 min-w-0 sm:col-span-1" :aria-label="t('userCustomization.methodValue')" :placeholder="t('userCustomization.methodValue')" />
          <div class="col-start-2 row-start-1 flex items-center gap-1 sm:col-start-3">
            <button type="button" class="icon-action" :title="t('userCustomization.moveUp')" :aria-label="t('userCustomization.moveUp')" :disabled="index === 0" @click="moveMethod(index, -1)"><Icon name="chevronUp" size="sm" /></button>
            <button type="button" class="icon-action" :title="t('userCustomization.moveDown')" :aria-label="t('userCustomization.moveDown')" :disabled="index === methods.length - 1" @click="moveMethod(index, 1)"><Icon name="chevronDown" size="sm" /></button>
            <button type="button" class="icon-action text-red-500" :title="t('common.delete')" :aria-label="t('common.delete')" @click="methods.splice(index, 1)"><Icon name="trash" size="sm" /></button>
          </div>
        </div>
      </fieldset>
    </section>

    <section>
      <form class="mb-4 flex flex-wrap items-center justify-between gap-3" @submit.prevent="page = 1; loadUsers()">
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('userCustomization.title') }}</h2>
        <div class="flex w-full gap-2 sm:w-auto">
          <input v-model="search" class="input min-w-0 flex-1 sm:w-64" maxlength="100" :aria-label="t('userCustomization.search')" :placeholder="t('userCustomization.search')" />
          <button type="submit" class="btn btn-secondary inline-flex items-center gap-2" :disabled="loading"><Icon name="search" size="sm" />{{ t('common.search') }}</button>
        </div>
      </form>
      <p v-if="loadFailed" role="alert" class="py-4 text-sm text-red-600">{{ t('userCustomization.loadFailed') }}</p>
      <div class="overflow-x-auto" :aria-busy="loading">
        <table class="w-full min-w-[780px] text-left text-sm">
          <thead class="border-y border-gray-200 text-xs text-gray-500 dark:border-dark-700">
            <tr><th class="p-3">{{ t('userCustomization.user') }}</th><th class="p-3">{{ t('balanceQuery.balance') }}</th><th class="p-3">{{ t('userCustomization.queryLink') }}</th><th class="p-3">{{ t('userCustomization.autoCredit') }}</th><th class="p-3">{{ t('common.actions') }}</th></tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-for="user in users" :key="user.user_id" :data-test="`custom-user-${user.user_id}`">
              <td class="max-w-56 p-3">
                <div class="break-words font-medium text-gray-900 dark:text-white">{{ userLabel(user) }}</div>
                <div v-if="user.username && user.email" class="break-all text-xs text-gray-500">{{ user.email }}</div>
                <div class="text-xs text-gray-500">ID {{ user.user_id }}</div>
              </td>
              <td class="p-3 font-medium tabular-nums text-emerald-700 dark:text-emerald-400">{{ money(user.balance) }}</td>
              <td class="p-3">
                <div class="flex items-center gap-2">
                  <span class="text-xs" :class="user.link_enabled ? 'text-emerald-600' : 'text-gray-500'">{{ t(user.has_link ? (user.link_enabled ? 'userCustomization.linkActive' : 'userCustomization.linkDisabled') : 'userCustomization.noLink') }}</span>
                  <button v-if="user.has_link" type="button" class="icon-action" :disabled="busy || user.status !== 'active'" :title="t('userCustomization.copyLink')" :aria-label="t('userCustomization.copyLink')" @click="copyLink(user)"><Icon name="copy" size="sm" /></button>
                </div>
                <div class="mt-1 flex gap-3 text-xs">
                  <button v-if="!user.has_link" type="button" class="text-primary-600" :disabled="busy || user.status !== 'active'" @click="generateLink(user)">{{ t('userCustomization.generateLink') }}</button>
                  <template v-else>
                    <button type="button" class="text-primary-600" :disabled="busy || user.status !== 'active'" @click="toggleLink(user)">{{ t(user.link_enabled ? 'userCustomization.disableLink' : 'userCustomization.enableLink') }}</button>
                    <button type="button" class="text-gray-500" :disabled="busy || user.status !== 'active'" @click="confirmation = { kind: 'rotate', user }">{{ t('userCustomization.resetLink') }}</button>
                  </template>
                </div>
              </td>
              <td class="p-3 text-xs">
                <span :class="user.credit_used_at ? 'text-amber-600' : 'text-gray-600 dark:text-gray-300'">{{ creditStatus(user) }}</span>
                <div class="mt-1 tabular-nums text-gray-500">&lt; {{ money(user.credit_threshold) }} / +{{ money(user.credit_amount) }}</div>
                <div v-if="user.credit_used_at" class="mt-1 text-gray-400">{{ formatDateTime(user.credit_used_at) }}</div>
              </td>
              <td class="p-3"><div class="flex items-center gap-2">
                <button type="button" class="icon-action" :disabled="busy || user.status !== 'active'" :title="t('userCustomization.configure')" :aria-label="t('userCustomization.configure')" @click="openEditor(user)"><Icon name="cog" size="sm" /></button>
                <button v-if="user.credit_used_at" type="button" class="btn btn-secondary text-xs" :disabled="busy || user.status !== 'active' || user.credit_amount <= 0" @click="confirmation = { kind: 'restore', user }">{{ t('userCustomization.restoreCredit') }}</button>
              </div></td>
            </tr>
            <tr v-if="!users.length"><td colspan="5" class="p-8 text-center text-gray-500">{{ t(loading ? 'common.loading' : 'userCustomization.noUsers') }}</td></tr>
          </tbody>
        </table>
      </div>
      <div class="mt-3 flex items-center justify-between text-sm text-gray-500">
        <span>{{ t('userCustomization.totalUsers', { count: total }) }}</span>
        <div class="flex items-center gap-2"><button type="button" class="icon-action" :disabled="loading || page <= 1" :title="t('common.previous')" :aria-label="t('common.previous')" @click="page--; loadUsers()"><Icon name="chevronLeft" size="sm" /></button><span class="min-w-8 text-center tabular-nums">{{ page }}</span><button type="button" class="icon-action" :disabled="loading || page * 20 >= total" :title="t('common.next')" :aria-label="t('common.next')" @click="page++; loadUsers()"><Icon name="chevronRight" size="sm" /></button></div>
      </div>
    </section>

    <BaseDialog :show="editing !== null" :title="t('userCustomization.configure')" @close="closeEditor">
      <form id="user-credit-settings" class="space-y-4" @submit.prevent="saveCredit">
        <div v-if="editing">
          <p class="break-words font-medium">{{ userLabel(editing) }} <span class="text-gray-500">ID {{ editing.user_id }}</span></p>
          <p v-if="editing.username && editing.email" class="break-all text-sm text-gray-500">{{ editing.email }}</p>
        </div>
        <fieldset :disabled="busy" class="space-y-4">
          <label class="flex items-center justify-between gap-4"><span>{{ t('userCustomization.autoCredit') }}</span><Toggle v-model="credit.auto_credit_enabled" /></label>
          <label class="block text-sm">{{ t('userCustomization.threshold') }}<input v-model.number="credit.credit_threshold" type="number" min="0" max="999999999999" step="0.00000001" required class="input mt-1 w-full" /></label>
          <label class="block text-sm">{{ t('userCustomization.amount') }}<input v-model.number="credit.credit_amount" type="number" :min="credit.auto_credit_enabled ? '0.00000001' : '0'" max="999999999999" step="0.00000001" required class="input mt-1 w-full" /></label>
        </fieldset>
      </form>
      <template #footer><button type="submit" form="user-credit-settings" class="btn btn-primary inline-flex items-center gap-2" :disabled="busy"><Icon name="check" size="sm" />{{ t('common.save') }}</button></template>
    </BaseDialog>
    <ConfirmDialog :show="confirmation !== null" :title="t(confirmation?.kind === 'restore' ? 'userCustomization.restoreCredit' : 'userCustomization.resetLink')" :message="confirmationMessage" :danger="true" @cancel="!busy && (confirmation = null)" @confirm="confirmAction" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import api, { type UserCustomization, type UserCreditSettings, type UserPaymentMethod } from '@/api/admin/userCustomizations'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const app = useAppStore()
const users = ref<UserCustomization[]>([])
const methods = ref<UserPaymentMethod[]>([])
const search = ref('')
const page = ref(1)
const total = ref(0)
const loading = ref(false)
const loadFailed = ref(false)
const methodsLoading = ref(true)
const methodsFailed = ref(false)
const methodsSaving = ref(false)
const busy = ref(false)
const editing = ref<UserCustomization | null>(null)
const credit = reactive<UserCreditSettings>({ auto_credit_enabled: false, credit_threshold: 1000, credit_amount: 0 })
const confirmation = ref<{ kind: 'restore' | 'rotate'; user: UserCustomization } | null>(null)
let requestVersion = 0
const userLabel = (user: UserCustomization) => user.username || user.email || `#${user.user_id}`
const money = (value: number) => `$${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 8 })}`
const creditStatus = (user: UserCustomization) => t(user.credit_used_at ? 'userCustomization.used' : user.auto_credit_enabled ? 'userCustomization.ready' : 'userCustomization.off')
const confirmationMessage = computed(() => {
  const action = confirmation.value
  if (!action) return ''
  return t(action.kind === 'restore' ? 'userCustomization.restoreConfirm' : 'userCustomization.resetLinkConfirm', {
    user: action.user.username && action.user.email ? `${userLabel(action.user)} (${action.user.email})` : userLabel(action.user), id: action.user.user_id,
    threshold: money(action.user.credit_threshold), amount: money(action.user.credit_amount)
  })
})

async function loadUsers() {
  const version = ++requestVersion
  loading.value = true
  loadFailed.value = false
  try {
    const result = await api.list(search.value.trim(), page.value)
    if (version === requestVersion) { users.value = result.items; total.value = result.total }
  } catch { if (version === requestVersion) loadFailed.value = true }
  finally { if (version === requestVersion) loading.value = false }
}
async function loadMethods() {
  methodsLoading.value = true
  methodsFailed.value = false
  try { methods.value = await api.paymentMethods() } catch { methodsFailed.value = true }
  finally { methodsLoading.value = false }
}
function moveMethod(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= methods.value.length) return
  const [item] = methods.value.splice(index, 1)
  methods.value.splice(target, 0, item)
}
async function saveMethods() {
  if (methodsSaving.value || methodsLoading.value || methodsFailed.value) return
  if (methods.value.some(item => !item.key.trim() || !item.value.trim())) { app.showError(t('userCustomization.invalidMethods')); return }
  methodsSaving.value = true
  try { methods.value = await api.savePaymentMethods(methods.value); app.showSuccess(t('userCustomization.saved')) }
  catch { app.showError(t('userCustomization.saveFailed')) }
  finally { methodsSaving.value = false }
}
function openEditor(user: UserCustomization) {
  editing.value = user
  Object.assign(credit, { auto_credit_enabled: user.auto_credit_enabled, credit_threshold: user.credit_threshold, credit_amount: user.credit_amount })
}
function closeEditor() { if (!busy.value) editing.value = null }
async function mutate(action: () => Promise<unknown>): Promise<boolean> {
  if (busy.value) return false
  busy.value = true
  try { await action(); await loadUsers(); return true }
  catch { app.showError(t('userCustomization.operationFailed')); await loadUsers(); return false }
  finally { busy.value = false }
}
async function saveCredit() {
  const user = editing.value
  if (!user) return
  if (await mutate(() => api.save(user.user_id, { ...credit }))) { editing.value = null; app.showSuccess(t('userCustomization.saved')) }
}
async function copyLink(user: UserCustomization) {
  if (busy.value) return
  busy.value = true
  try { await navigator.clipboard.writeText(await api.getLink(user.user_id)); app.showSuccess(t('userCustomization.copied')) }
  catch { app.showError(t('userCustomization.copyFailed')) }
  finally { busy.value = false }
}
async function generateLink(user: UserCustomization) { await mutate(() => api.rotateLink(user)) }
async function toggleLink(user: UserCustomization) { await mutate(() => api.setLinkEnabled(user, !user.link_enabled)) }
async function confirmAction() {
  const action = confirmation.value
  if (!action || busy.value) return
  if (await mutate(() => action.kind === 'restore' ? api.restoreCredit(action.user) : api.rotateLink(action.user))) confirmation.value = null
}
onMounted(() => { void loadUsers(); void loadMethods() })
onBeforeUnmount(() => { requestVersion++ })
</script>

<style scoped>
.icon-action { display: inline-flex; width: 32px; height: 32px; flex-shrink: 0; align-items: center; justify-content: center; border-radius: 4px; }
.icon-action:hover:not(:disabled) { background: rgb(148 163 184 / 15%); }
button:disabled { opacity: .45; cursor: not-allowed; }
</style>
