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
              {{ localText('模型广场', 'Model Marketplace') }}
            </div>
          </div>
        </router-link>

        <div class="flex items-center gap-2">
          <button class="btn btn-secondary px-3 py-2" :disabled="loading" @click="loadMarketplace">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="hidden sm:inline">{{ localText('刷新', 'Refresh') }}</span>
          </button>
          <router-link to="/home" class="btn btn-primary px-3 py-2">
            <Icon name="home" size="sm" />
            <span class="hidden sm:inline">{{ localText('首页', 'Home') }}</span>
          </router-link>
        </div>
      </nav>
    </header>

    <main :class="mainClass">
      <section class="mb-8 grid gap-6 lg:grid-cols-[1fr_380px] lg:items-end">
        <div>
          <div class="mb-4 inline-flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
            <PulseDot tone="primary" />
            <span>AI API CATALOG</span>
          </div>
          <h1 class="text-4xl font-bold tracking-normal text-gray-900 dark:text-white md:text-5xl">
            {{ localText('模型广场', 'Model Marketplace') }}
          </h1>
          <p class="mt-4 max-w-3xl text-base leading-7 text-slate-600 dark:text-dark-300">
            {{ localText('实时查看公开分组支持的模型，以及可以直接调用的对话与生成接口。', 'Browse live public group models and the conversation or generation endpoints ready to call.') }}
          </p>
          <div v-if="marketplace?.intro" class="mt-5 max-w-3xl rounded-xl border border-primary-200/60 bg-white/75 p-4 text-sm leading-6 text-slate-700 shadow-sm backdrop-blur dark:border-primary-400/25 dark:bg-dark-900/70 dark:text-slate-200">
            <p class="whitespace-pre-line">{{ marketplace.intro }}</p>
          </div>
        </div>

        <div class="rounded-2xl border border-primary-200/60 bg-white/75 p-5 shadow-xl shadow-primary-500/10 backdrop-blur dark:border-primary-400/25 dark:bg-[#0b1420]/80">
          <div class="flex items-center justify-between gap-3">
            <div class="font-mono text-[10px] font-semibold uppercase tracking-[0.2em] text-slate-500 dark:text-slate-400">
              {{ localText('实时快照', 'Live Snapshot') }}
            </div>
            <PulseDot tone="success" />
          </div>
          <div class="mt-5 grid grid-cols-3 gap-3">
            <div class="market-stat">
              <div class="market-stat-value">{{ visibleGroups.length }}</div>
              <div class="market-stat-label">{{ localText('分组', 'Groups') }}</div>
            </div>
            <div class="market-stat">
              <div class="market-stat-value">{{ totalModels }}</div>
              <div class="market-stat-label">{{ localText('模型', 'Models') }}</div>
            </div>
            <div class="market-stat">
              <div class="market-stat-value">{{ totalFormats }}</div>
              <div class="market-stat-label">{{ localText('接口', 'APIs') }}</div>
            </div>
          </div>
          <div class="mt-4 font-mono text-[11px] text-slate-500 dark:text-slate-400">
            {{ localText('更新时间', 'Updated') }}: {{ formattedGeneratedAt }}
          </div>
        </div>
      </section>

      <section class="mb-6 grid gap-3 md:grid-cols-[1fr_auto]">
        <label class="relative block">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-primary-500" />
          <input
            v-model="query"
            class="h-11 w-full rounded-xl border border-primary-200/60 bg-white/85 pl-9 pr-3 text-sm outline-none backdrop-blur transition focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-primary-400/25 dark:bg-[#0b1420]/80 dark:text-white"
            :placeholder="localText('搜索分组、模型或接口', 'Search groups, models, or endpoints')"
          />
        </label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="platform in platformFilters"
            :key="platform.value"
            class="h-11 rounded-xl border px-3 text-sm font-semibold transition-colors"
            :class="selectedPlatform === platform.value ? 'border-primary-500 bg-primary-500 text-white shadow-lg shadow-primary-500/25' : 'border-primary-200/60 bg-white/75 text-slate-700 hover:border-primary-400 hover:bg-white dark:border-primary-400/25 dark:bg-[#0b1420]/80 dark:text-slate-200 dark:hover:bg-dark-800'"
            @click="selectedPlatform = platform.value"
          >
            {{ platform.label }}
          </button>
        </div>
      </section>

      <div v-if="loading" class="market-empty">
        {{ localText('正在加载模型广场...', 'Loading model marketplace...') }}
      </div>
      <div v-else-if="error" class="rounded-xl border border-rose-200 bg-rose-50/90 p-6 text-sm text-rose-700 backdrop-blur dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200">
        {{ error }}
      </div>
      <div v-else-if="marketplace && !marketplace.enabled" class="market-empty">
        <Icon name="lock" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('模型广场未启用', 'Model marketplace is disabled') }}</h2>
      </div>
      <div v-else-if="filteredGroups.length === 0" class="market-empty">
        <Icon name="inbox" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('暂无可展示模型', 'No models to show') }}</h2>
        <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
          {{ localText('请调整搜索条件，或等待管理员配置公开分组。', 'Adjust filters or wait for an administrator to configure public groups.') }}
        </p>
      </div>

      <section v-else class="grid gap-5">
        <article v-for="group in filteredGroups" :key="group.id" class="market-group">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-xl font-semibold text-slate-950 dark:text-white">{{ group.name }}</h2>
                <span class="market-badge">{{ platformLabel(group.platform) }}</span>
                <span v-if="group.is_exclusive" class="market-badge market-badge-warn">{{ localText('专属展示', 'Exclusive') }}</span>
                <span v-if="group.subscription_type === 'subscription'" class="market-badge market-badge-info">{{ localText('订阅分组', 'Subscription') }}</span>
              </div>
              <p v-if="group.description" class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">{{ group.description }}</p>
            </div>
            <div class="shrink-0 rounded-full border border-primary-200/60 bg-primary-50/80 px-3 py-1 font-mono text-xs font-semibold text-primary-700 dark:border-primary-400/25 dark:bg-primary-500/10 dark:text-primary-200">
              {{ group.models.length }} {{ localText('个模型', 'models') }} · {{ group.request_formats.length }} {{ localText('个接口', 'APIs') }}
            </div>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <span v-for="model in group.models" :key="model" class="market-model-chip">{{ model }}</span>
            <span v-if="group.models.length === 0" class="text-sm text-slate-500 dark:text-slate-400">
              {{ localText('暂无可展示模型', 'No visible models') }}
            </span>
          </div>

          <div class="mt-5 divide-y divide-primary-200/50 overflow-hidden rounded-xl border border-primary-200/60 bg-white/60 dark:divide-primary-400/15 dark:border-primary-400/20 dark:bg-[#07111f]/70">
            <div v-for="format in group.request_formats" :key="`${group.id}-${format.method}-${format.path}`" class="api-row">
              <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
                <button type="button" class="min-w-0 text-left" @click="toggleFormat(group.id, format)">
                  <div class="flex min-w-0 flex-wrap items-center gap-2">
                    <span class="method-pill">{{ format.method }}</span>
                    <span class="truncate font-mono text-sm font-semibold text-slate-900 dark:text-white">{{ resolvePath(format, group.models[0]) }}</span>
                  </div>
                  <div class="mt-1 flex items-center gap-2 text-sm text-slate-500 dark:text-slate-400">
                    <Icon :name="isExpanded(group.id, format) ? 'chevronUp' : 'chevronDown'" size="xs" />
                    <span>{{ format.name }}</span>
                  </div>
                </button>
                <div class="flex shrink-0 gap-2">
                  <button class="btn btn-secondary px-3 py-2 text-xs" @click="copyFormat(format, group.models[0])">
                    <Icon name="copy" size="sm" />
                    {{ localText('复制', 'Copy') }}
                  </button>
                  <button class="btn btn-ghost px-3 py-2 text-xs" @click="toggleFormat(group.id, format)">
                    {{ isExpanded(group.id, format) ? localText('收起参数', 'Hide body') : localText('查看参数', 'Show body') }}
                  </button>
                </div>
              </div>
              <transition name="format-body">
                <pre v-if="isExpanded(group.id, format)" class="mt-4 max-h-72 overflow-auto rounded-lg border border-slate-700/80 bg-slate-950 p-4 text-xs leading-5 text-slate-100 shadow-inner"><code>{{ formatExample(format, group.models[0]) }}</code></pre>
              </transition>
            </div>
          </div>
        </article>
      </section>
    </main>
  </component>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { modelMarketplaceAPI } from '@/api'
import type { ModelMarketplaceRequestFormat, ModelMarketplaceResponse } from '@/types'
import BackgroundFX from '@/components/common/BackgroundFX.vue'
import PulseDot from '@/components/common/PulseDot.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'

const { locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const error = ref('')
const marketplace = ref<ModelMarketplaceResponse | null>(null)
const query = ref('')
const selectedPlatform = ref('all')
const expandedFormats = ref<Set<string>>(new Set())

const isAuthenticated = computed(() => authStore.isAuthenticated)
const publicShellClass = computed(() => isAuthenticated.value ? '' : 'relative min-h-screen overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 text-slate-900 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950 dark:text-white')
const mainClass = computed(() => isAuthenticated.value
  ? 'relative z-10 mx-auto max-w-7xl pb-6'
  : 'relative z-10 mx-auto max-w-7xl px-4 pb-12 pt-8 sm:px-6 lg:pt-12'
)
const isZhLocale = computed(() => locale.value.startsWith('zh'))
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')

const visibleGroups = computed(() => marketplace.value?.groups ?? [])
const platforms = computed(() => Array.from(new Set(visibleGroups.value.map((group) => group.platform))).sort())
const totalModels = computed(() => visibleGroups.value.reduce((sum, group) => sum + group.models.length, 0))
const totalFormats = computed(() => visibleGroups.value.reduce((sum, group) => sum + group.request_formats.length, 0))
const formattedGeneratedAt = computed(() => {
  if (!marketplace.value?.generated_at) return '-'
  return new Date(marketplace.value.generated_at).toLocaleString()
})

const platformFilters = computed(() => [
  { value: 'all', label: localText('全部平台', 'All') },
  ...platforms.value.map((platform) => ({ value: platform, label: platformLabel(platform) }))
])

const filteredGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  return visibleGroups.value.filter((group) => {
    if (selectedPlatform.value !== 'all' && group.platform !== selectedPlatform.value) {
      return false
    }
    if (!q) return true
    const haystack = [
      group.name,
      group.description,
      group.platform,
      ...group.models,
      ...group.request_formats.flatMap((format) => [format.name, format.method, format.path])
    ].join('\n').toLowerCase()
    return haystack.includes(q)
  })
})

function localText(zh: string, en: string): string {
  return isZhLocale.value ? zh : en
}

function platformLabel(platform: string): string {
  switch (platform) {
    case 'openai':
      return 'OpenAI'
    case 'gemini':
      return 'Gemini'
    case 'antigravity':
      return 'Antigravity'
    default:
      return 'Anthropic'
  }
}

function formatKey(groupID: number, format: ModelMarketplaceRequestFormat): string {
  return `${groupID}:${format.method}:${format.path}`
}

function isExpanded(groupID: number, format: ModelMarketplaceRequestFormat): boolean {
  return expandedFormats.value.has(formatKey(groupID, format))
}

function toggleFormat(groupID: number, format: ModelMarketplaceRequestFormat): void {
  const next = new Set(expandedFormats.value)
  const key = formatKey(groupID, format)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedFormats.value = next
}

function resolvePath(format: ModelMarketplaceRequestFormat, model?: string): string {
  return format.path.split('{model}').join(model || '{model}')
}

function formatExample(format: ModelMarketplaceRequestFormat, model?: string): string {
  const resolvedModel = model || '{model}'
  const lines = [
    `${format.method} ${format.path.split('{model}').join(resolvedModel)}`,
    'Authorization: Bearer sk-...',
  ]
  if (format.content_type) {
    lines.push(`Content-Type: ${format.content_type}`)
  }
  if (format.body) {
    lines.push('', format.body.split('{model}').join(resolvedModel))
  }
  return lines.join('\n')
}

async function copyFormat(format: ModelMarketplaceRequestFormat, model?: string): Promise<void> {
  await copyToClipboard(formatExample(format, model))
}

async function loadMarketplace(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    marketplace.value = await modelMarketplaceAPI.getModelMarketplace()
  } catch (err: unknown) {
    error.value = extractApiErrorMessage(err, localText('加载模型广场失败', 'Failed to load model marketplace'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  if (!appStore.cachedPublicSettings && !appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  loadMarketplace()
})
</script>

<style scoped>
.market-stat {
  border-radius: 0.75rem;
  border: 1px solid rgba(20, 184, 166, 0.18);
  background: rgba(255, 255, 255, 0.68);
  padding: 0.85rem;
}

:global(.dark) .market-stat {
  background: rgba(15, 23, 42, 0.62);
}

.market-stat-value {
  font-size: 1.6rem;
  line-height: 1;
  font-weight: 800;
  color: rgb(15 23 42);
}

:global(.dark) .market-stat-value {
  color: white;
}

.market-stat-label {
  margin-top: 0.35rem;
  font-size: 0.7rem;
  color: rgb(100 116 139);
}

.market-empty,
.market-group {
  border-radius: 1rem;
  border: 1px solid rgba(20, 184, 166, 0.22);
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 22px 60px rgba(15, 68, 112, 0.1);
  backdrop-filter: blur(16px);
}

.market-empty {
  padding: 2.5rem;
  text-align: center;
}

.market-group {
  padding: 1.25rem;
}

:global(.dark) .market-empty,
:global(.dark) .market-group {
  border-color: rgba(75, 181, 255, 0.22);
  background: rgba(11, 20, 32, 0.78);
  box-shadow: 0 22px 70px rgba(0, 0, 0, 0.28);
}

.market-badge {
  border-radius: 0.5rem;
  background: rgba(20, 184, 166, 0.12);
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 700;
  color: rgb(15 118 110);
}

.market-badge-warn {
  background: rgba(245, 158, 11, 0.14);
  color: rgb(180 83 9);
}

.market-badge-info {
  background: rgba(14, 165, 233, 0.14);
  color: rgb(3 105 161);
}

:global(.dark) .market-badge {
  color: rgb(153 246 228);
}

:global(.dark) .market-badge-warn {
  color: rgb(253 230 138);
}

:global(.dark) .market-badge-info {
  color: rgb(186 230 253);
}

.market-model-chip {
  border-radius: 0.5rem;
  border: 1px solid rgba(20, 184, 166, 0.2);
  background: rgba(248, 250, 252, 0.86);
  padding: 0.3rem 0.6rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.75rem;
  color: rgb(51 65 85);
}

:global(.dark) .market-model-chip {
  border-color: rgba(75, 181, 255, 0.22);
  background: rgba(2, 6, 23, 0.7);
  color: rgb(226 232 240);
}

.api-row {
  padding: 1rem;
}

.method-pill {
  border-radius: 999px;
  background: linear-gradient(135deg, rgb(45 212 191), rgb(14 165 233));
  padding: 0.2rem 0.55rem;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 0.7rem;
  font-weight: 800;
  color: white;
}

.format-body-enter-active,
.format-body-leave-active {
  transition: opacity 180ms ease, transform 180ms ease;
}

.format-body-enter-from,
.format-body-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
