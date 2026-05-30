<template>
  <component :is="isAuthenticated ? AppLayout : 'div'" :class="publicShellClass">
    <template v-if="!isAuthenticated">
      <BackgroundFX variant="home" :density="0.75" />
      <div class="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.035)_1px,transparent_1px)] bg-[size:64px_64px]"></div>
    </template>

    <header v-if="!isAuthenticated" class="relative z-20 px-4 py-4 sm:px-6">
      <nav class="mx-auto flex max-w-7xl items-center justify-between gap-4">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl border border-primary-200/70 bg-white/95 shadow-glow dark:border-primary-400/30 dark:bg-[#0b1420]">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold text-slate-950 dark:text-white">{{ siteName }}</div>
            <div class="font-mono text-[10px] font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
              {{ localText('模型定价', 'Model Pricing') }}
            </div>
          </div>
        </router-link>

        <div class="flex items-center gap-2">
          <button class="btn btn-secondary px-3 py-2" :disabled="loading" @click="loadPricing">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="hidden sm:inline">{{ localText('刷新', 'Refresh') }}</span>
          </button>
          <router-link to="/models" class="btn btn-secondary px-3 py-2">
            <Icon name="cube" size="sm" />
            <span class="hidden sm:inline">{{ localText('模型广场', 'Models') }}</span>
          </router-link>
          <router-link to="/home" class="btn btn-primary px-3 py-2">
            <Icon name="home" size="sm" />
            <span class="hidden sm:inline">{{ localText('首页', 'Home') }}</span>
          </router-link>
        </div>
      </nav>
    </header>

    <main :class="mainClass">
      <section class="mb-8 grid gap-6 lg:grid-cols-[1fr_360px] lg:items-end">
        <div>
          <div class="mb-4 inline-flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
            <PulseDot tone="primary" />
            <span>MODEL PRICING</span>
          </div>
          <h1 class="text-4xl font-bold tracking-normal text-gray-900 dark:text-white md:text-5xl">
            {{ localText('模型定价', 'Model Pricing') }}
          </h1>
          <p class="mt-4 max-w-3xl text-base leading-7 text-slate-600 dark:text-dark-300">
            {{ localText('查看当前可购买的模型订阅套餐、倍率和额度配置。套餐内容由管理员维护，页面会读取最新配置。', 'View live subscription plans, rates, and quota limits maintained by the administrator.') }}
          </p>
        </div>

        <div class="rounded-2xl border border-primary-200/60 bg-white/75 p-5 shadow-xl shadow-primary-500/10 backdrop-blur dark:border-primary-400/25 dark:bg-[#0b1420]/80">
          <div class="flex items-center justify-between gap-3">
            <div class="font-mono text-[10px] font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-slate-400">
              {{ localText('实时价格', 'Live Pricing') }}
            </div>
            <PulseDot :tone="pricing?.enabled ? 'success' : 'warning'" />
          </div>
          <div class="mt-5 grid grid-cols-2 gap-3">
            <div class="pricing-stat">
              <div class="pricing-stat-value">{{ visiblePlans.length }}</div>
              <div class="pricing-stat-label">{{ localText('套餐', 'Plans') }}</div>
            </div>
            <div class="pricing-stat">
              <div class="pricing-stat-value">{{ platformCount }}</div>
              <div class="pricing-stat-label">{{ localText('平台', 'Platforms') }}</div>
            </div>
          </div>
          <div class="mt-4 font-mono text-[11px] text-slate-500 dark:text-slate-400">
            {{ localText('更新时间', 'Updated') }}: {{ formattedGeneratedAt }}
          </div>
        </div>
      </section>

      <div v-if="loading" class="pricing-empty">
        {{ localText('正在加载模型定价...', 'Loading model pricing...') }}
      </div>
      <div v-else-if="error" class="rounded-xl border border-rose-200 bg-rose-50/90 p-6 text-sm text-rose-700 backdrop-blur dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200">
        {{ error }}
      </div>
      <div v-else-if="pricing && !pricing.enabled" class="pricing-empty">
        <Icon name="lock" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('模型定价暂未开放', 'Model pricing is disabled') }}</h2>
      </div>
      <div v-else-if="visiblePlans.length === 0" class="pricing-empty">
        <Icon name="inbox" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('暂无可展示套餐', 'No plans to show') }}</h2>
        <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
          {{ localText('请等待管理员上架订阅套餐。', 'Please wait for an administrator to publish subscription plans.') }}
        </p>
      </div>

      <section v-else class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="plan in visiblePlans" :key="plan.id" class="pricing-card">
          <div :class="['h-1.5', platformAccentBarClass(plan.group_platform)]"></div>
          <div class="flex flex-1 flex-col p-5">
            <div class="mb-4 flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h2 class="truncate text-lg font-bold text-slate-950 dark:text-white">{{ plan.name }}</h2>
                  <span :class="['rounded-full px-2 py-0.5 text-[11px] font-semibold', platformBadgeLightClass(plan.group_platform)]">
                    {{ platformLabel(plan.group_platform) }}
                  </span>
                </div>
                <p v-if="plan.description" class="mt-2 line-clamp-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                  {{ plan.description }}
                </p>
              </div>
              <div class="shrink-0 text-right">
                <div class="flex items-baseline justify-end gap-1">
                  <span class="text-xs text-slate-400">$</span>
                  <span :class="['text-3xl font-extrabold tracking-normal', platformTextClass(plan.group_platform)]">{{ formatPrice(plan.price) }}</span>
                </div>
                <div class="text-xs text-slate-400 dark:text-slate-500">/ {{ validitySuffix(plan) }}</div>
                <div v-if="plan.original_price" class="mt-1 flex items-center justify-end gap-1.5">
                  <span class="text-xs text-slate-400 line-through dark:text-slate-500">${{ formatPrice(plan.original_price) }}</span>
                  <span :class="['rounded px-1 py-0.5 text-[10px] font-semibold', platformDiscountClass(plan.group_platform)]">{{ discountText(plan) }}</span>
                </div>
              </div>
            </div>

            <div class="mb-4 grid grid-cols-2 gap-x-3 gap-y-2 rounded-xl bg-slate-50 px-3 py-3 text-xs dark:bg-[#07111f]/80">
              <div class="flex items-center justify-between gap-2">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('倍率', 'Rate') }}</span>
                <span class="font-semibold text-slate-700 dark:text-slate-200">×{{ formatRate(plan.rate_multiplier) }}</span>
              </div>
              <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between gap-2">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('日限额', 'Daily') }}</span>
                <span class="font-semibold text-slate-700 dark:text-slate-200">${{ formatPrice(plan.daily_limit_usd) }}</span>
              </div>
              <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between gap-2">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('周限额', 'Weekly') }}</span>
                <span class="font-semibold text-slate-700 dark:text-slate-200">${{ formatPrice(plan.weekly_limit_usd) }}</span>
              </div>
              <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between gap-2">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('月限额', 'Monthly') }}</span>
                <span class="font-semibold text-slate-700 dark:text-slate-200">${{ formatPrice(plan.monthly_limit_usd) }}</span>
              </div>
              <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between gap-2">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('配额', 'Quota') }}</span>
                <span class="font-semibold text-slate-700 dark:text-slate-200">{{ localText('无限制', 'Unlimited') }}</span>
              </div>
              <div v-if="modelScopeLabels(plan).length > 0" class="col-span-2 flex items-center justify-between gap-3">
                <span class="text-slate-400 dark:text-slate-500">{{ localText('模型', 'Models') }}</span>
                <div class="flex flex-wrap justify-end gap-1">
                  <span v-for="scope in modelScopeLabels(plan)" :key="scope" class="rounded bg-slate-200/80 px-1.5 py-0.5 text-[10px] font-semibold text-slate-600 dark:bg-dark-600 dark:text-slate-200">
                    {{ scope }}
                  </span>
                </div>
              </div>
            </div>

            <div v-if="plan.features.length > 0" class="mb-5 space-y-2">
              <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-2">
                <Icon name="check" size="sm" :class="['mt-0.5 shrink-0', platformIconClass(plan.group_platform)]" :stroke-width="2.5" />
                <span class="text-sm leading-5 text-slate-600 dark:text-slate-300">{{ feature }}</span>
              </div>
            </div>

            <div class="flex-1"></div>

            <router-link :to="purchaseTarget(plan)" :class="['inline-flex w-full items-center justify-center gap-2 rounded-xl py-2.5 text-sm font-semibold transition-all active:scale-[0.98]', platformButtonClass(plan.group_platform)]">
              <Icon name="login" size="sm" :stroke-width="2" />
              {{ isAuthenticated ? localText('立即开通', 'Subscribe Now') : localText('登录后开通', 'Login to Subscribe') }}
            </router-link>
          </div>
        </article>
      </section>

      <section v-if="!error && pricing?.enabled && (pricing.help_text || pricing.help_image_url)" class="mt-8 overflow-hidden rounded-2xl border border-primary-200/60 bg-white/75 p-5 shadow-xl shadow-primary-500/10 backdrop-blur dark:border-primary-400/25 dark:bg-[#0b1420]/80">
        <div class="flex flex-col items-center gap-4">
          <img
            v-if="pricing.help_image_url"
            :src="pricing.help_image_url"
            alt=""
            class="max-h-56 max-w-full rounded-xl object-contain"
          />
          <p v-if="pricing.help_text" class="max-w-4xl text-center text-sm leading-6 text-slate-600 dark:text-slate-300">
            {{ pricing.help_text }}
          </p>
        </div>
      </section>
    </main>
  </component>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { modelPricingAPI } from '@/api'
import type { PublicModelPricingPlan, PublicModelPricingResponse } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import BackgroundFX from '@/components/common/BackgroundFX.vue'
import PulseDot from '@/components/common/PulseDot.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  platformAccentBarClass,
  platformBadgeLightClass,
  platformButtonClass,
  platformDiscountClass,
  platformIconClass,
  platformLabel,
  platformTextClass,
} from '@/utils/platformColors'

const { locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(false)
const error = ref('')
const pricing = ref<PublicModelPricingResponse | null>(null)

const isAuthenticated = computed(() => authStore.isAuthenticated)
const publicShellClass = computed(() => isAuthenticated.value ? '' : 'relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 text-slate-900 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950 dark:text-white')
const mainClass = computed(() => isAuthenticated.value
  ? 'relative z-10 mx-auto max-w-7xl pb-6'
  : 'relative z-10 mx-auto max-w-7xl px-4 pb-12 pt-8 sm:px-6 lg:pt-12'
)
const isZhLocale = computed(() => locale.value.startsWith('zh'))
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const visiblePlans = computed(() => pricing.value?.plans ?? [])
const platformCount = computed(() => new Set(visiblePlans.value.map((plan) => plan.group_platform || 'api')).size)
const formattedGeneratedAt = computed(() => {
  if (!pricing.value?.generated_at) return '-'
  return new Date(pricing.value.generated_at).toLocaleString()
})

function localText(zh: string, en: string): string {
  return isZhLocale.value ? zh : en
}

function formatPrice(value: number): string {
  if (!Number.isFinite(value)) return '0'
  return Number(value.toFixed(2)).toString()
}

function formatRate(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '1'
  return Number(value.toPrecision(10)).toString()
}

function validitySuffix(plan: PublicModelPricingPlan): string {
  const unit = plan.validity_unit || 'day'
  if (unit === 'month') return localText('月', 'month')
  if (unit === 'year') return localText('年', 'year')
  return isZhLocale.value ? `${plan.validity_days}天` : `${plan.validity_days} days`
}

function discountText(plan: PublicModelPricingPlan): string {
  if (!plan.original_price || plan.original_price <= 0) return ''
  const pct = Math.round((1 - plan.price / plan.original_price) * 100)
  return pct > 0 ? `-${pct}%` : ''
}

const MODEL_SCOPE_LABELS: Record<string, string> = {
  claude: 'Claude',
  gemini_text: 'Gemini',
  gemini_image: 'Imagen',
}

function modelScopeLabels(plan: PublicModelPricingPlan): string[] {
  if (plan.group_platform !== 'antigravity') return []
  const scopes = plan.supported_model_scopes || []
  return scopes.map((scope) => MODEL_SCOPE_LABELS[scope] || scope)
}

function purchaseTarget(plan: PublicModelPricingPlan) {
  const purchaseQuery = { tab: 'subscription', plan_id: String(plan.id) }
  if (isAuthenticated.value) {
    return { path: '/purchase', query: purchaseQuery }
  }
  return {
    path: '/login',
    query: {
      redirect: `/purchase?tab=subscription&plan_id=${plan.id}`,
    },
  }
}

async function loadPricing(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    pricing.value = await modelPricingAPI.getModelPricing()
  } catch (err) {
    error.value = extractApiErrorMessage(err, localText('加载模型定价失败', 'Failed to load model pricing'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings().catch(() => {})
  }
  loadPricing()
})
</script>

<style scoped>
.pricing-stat {
  border-radius: 0.85rem;
  border: 1px solid rgba(20, 184, 166, 0.18);
  background: rgba(255, 255, 255, 0.72);
  padding: 0.85rem;
}

:global(.dark) .pricing-stat {
  border-color: rgba(45, 212, 191, 0.18);
  background: rgba(7, 17, 31, 0.82);
}

.pricing-stat-value {
  font-family: Bahnschrift, 'DIN Alternate', 'Arial Narrow', system-ui, sans-serif;
  font-size: 1.75rem;
  font-weight: 800;
  letter-spacing: 0;
  color: #0f766e;
}

:global(.dark) .pricing-stat-value {
  color: #99f6e4;
}

.pricing-stat-label {
  margin-top: 0.15rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: #64748b;
}

:global(.dark) .pricing-stat-label {
  color: #94a3b8;
}

.pricing-empty {
  border: 1px solid rgba(20, 184, 166, 0.18);
  border-radius: 1rem;
  background: rgba(255, 255, 255, 0.72);
  padding: 4rem 1.5rem;
  text-align: center;
  color: #475569;
  backdrop-filter: blur(14px);
}

:global(.dark) .pricing-empty {
  border-color: rgba(45, 212, 191, 0.18);
  background: rgba(7, 17, 31, 0.76);
  color: #cbd5e1;
}

.pricing-card {
  display: flex;
  min-height: 100%;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(20, 184, 166, 0.18);
  border-radius: 1rem;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: 0 18px 42px rgba(15, 68, 112, 0.1);
  backdrop-filter: blur(14px);
  transition: transform 220ms ease, box-shadow 220ms ease, border-color 220ms ease;
}

.pricing-card:hover {
  transform: translateY(-2px);
  border-color: rgba(20, 184, 166, 0.34);
  box-shadow: 0 24px 52px rgba(15, 68, 112, 0.16);
}

:global(.dark) .pricing-card {
  border-color: rgba(45, 212, 191, 0.18);
  background: rgba(7, 17, 31, 0.82);
  box-shadow: 0 20px 44px rgba(0, 0, 0, 0.34);
}
</style>
