<template>
  <div class="min-h-screen bg-slate-50 text-slate-900 dark:bg-dark-950 dark:text-white">
    <header class="border-b border-slate-200 bg-white/90 backdrop-blur dark:border-dark-800 dark:bg-dark-900/90">
      <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <router-link to="/home" class="flex min-w-0 items-center gap-3">
          <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-9 w-9 rounded-lg object-contain" />
          <div class="min-w-0">
            <div class="truncate text-sm font-semibold">{{ siteName }}</div>
            <div class="text-xs text-slate-500 dark:text-slate-400">{{ localText('模型广场', 'Model Marketplace') }}</div>
          </div>
        </router-link>
        <div class="flex items-center gap-2">
          <button
            class="inline-flex h-9 items-center gap-2 rounded-lg border border-slate-200 px-3 text-sm text-slate-700 hover:bg-slate-100 disabled:opacity-60 dark:border-dark-700 dark:text-slate-200 dark:hover:bg-dark-800"
            :disabled="loading"
            @click="loadMarketplace"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span class="hidden sm:inline">{{ localText('刷新', 'Refresh') }}</span>
          </button>
          <router-link
            to="/home"
            class="inline-flex h-9 items-center gap-2 rounded-lg bg-slate-900 px-3 text-sm text-white hover:bg-slate-800 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
          >
            <Icon name="home" size="sm" />
            <span class="hidden sm:inline">{{ localText('首页', 'Home') }}</span>
          </router-link>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:py-10">
      <section class="mb-7 grid gap-5 lg:grid-cols-[1fr_360px]">
        <div>
          <p class="mb-3 text-xs font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
            AI API CATALOG
          </p>
          <h1 class="text-3xl font-bold tracking-normal text-slate-950 dark:text-white sm:text-4xl">
            {{ localText('模型广场', 'Model Marketplace') }}
          </h1>
          <p class="mt-3 max-w-3xl text-sm leading-6 text-slate-600 dark:text-slate-300">
            {{ localText('查看当前公开展示分组支持的模型，以及对应可请求的接口格式。', 'Browse the models currently exposed by public groups and the API formats they accept.') }}
          </p>
          <p v-if="marketplace?.intro" class="mt-4 whitespace-pre-line rounded-lg border border-slate-200 bg-white p-4 text-sm leading-6 text-slate-700 dark:border-dark-800 dark:bg-dark-900 dark:text-slate-200">
            {{ marketplace.intro }}
          </p>
        </div>

        <div class="rounded-lg border border-slate-200 bg-white p-4 dark:border-dark-800 dark:bg-dark-900">
          <div class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
            {{ localText('实时快照', 'Live Snapshot') }}
          </div>
          <div class="mt-4 grid grid-cols-3 gap-3">
            <div>
              <div class="text-2xl font-semibold">{{ visibleGroups.length }}</div>
              <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ localText('分组', 'Groups') }}</div>
            </div>
            <div>
              <div class="text-2xl font-semibold">{{ totalModels }}</div>
              <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ localText('模型', 'Models') }}</div>
            </div>
            <div>
              <div class="text-2xl font-semibold">{{ platforms.length }}</div>
              <div class="mt-1 text-xs text-slate-500 dark:text-slate-400">{{ localText('平台', 'Platforms') }}</div>
            </div>
          </div>
          <div class="mt-4 text-xs text-slate-500 dark:text-slate-400">
            {{ localText('更新时间', 'Updated') }}: {{ formattedGeneratedAt }}
          </div>
        </div>
      </section>

      <section class="mb-6 grid gap-3 md:grid-cols-[1fr_auto]">
        <label class="relative block">
          <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            v-model="query"
            class="h-10 w-full rounded-lg border border-slate-200 bg-white pl-9 pr-3 text-sm outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-dark-700 dark:bg-dark-900"
            :placeholder="localText('搜索分组、模型或接口', 'Search groups, models, or endpoints')"
          />
        </label>
        <div class="flex flex-wrap gap-2">
          <button
            v-for="platform in platformFilters"
            :key="platform.value"
            class="h-10 rounded-lg border px-3 text-sm transition-colors"
            :class="selectedPlatform === platform.value ? 'border-primary-500 bg-primary-500 text-white' : 'border-slate-200 bg-white text-slate-700 hover:bg-slate-100 dark:border-dark-700 dark:bg-dark-900 dark:text-slate-200 dark:hover:bg-dark-800'"
            @click="selectedPlatform = platform.value"
          >
            {{ platform.label }}
          </button>
        </div>
      </section>

      <div v-if="loading" class="rounded-lg border border-slate-200 bg-white p-10 text-center text-sm text-slate-500 dark:border-dark-800 dark:bg-dark-900 dark:text-slate-400">
        {{ localText('正在加载模型广场...', 'Loading model marketplace...') }}
      </div>
      <div v-else-if="error" class="rounded-lg border border-rose-200 bg-rose-50 p-6 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-200">
        {{ error }}
      </div>
      <div v-else-if="marketplace && !marketplace.enabled" class="rounded-lg border border-slate-200 bg-white p-10 text-center dark:border-dark-800 dark:bg-dark-900">
        <Icon name="lock" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('模型广场未启用', 'Model marketplace is disabled') }}</h2>
      </div>
      <div v-else-if="filteredGroups.length === 0" class="rounded-lg border border-slate-200 bg-white p-10 text-center dark:border-dark-800 dark:bg-dark-900">
        <Icon name="inbox" size="xl" class="mx-auto text-slate-400" />
        <h2 class="mt-4 text-lg font-semibold">{{ localText('暂无可展示模型', 'No models to show') }}</h2>
        <p class="mt-2 text-sm text-slate-500 dark:text-slate-400">
          {{ localText('请调整搜索条件，或等待管理员配置公开分组。', 'Adjust filters or wait for an administrator to configure public groups.') }}
        </p>
      </div>

      <section v-else class="grid gap-4">
        <article
          v-for="group in filteredGroups"
          :key="group.id"
          class="rounded-lg border border-slate-200 bg-white p-5 shadow-sm dark:border-dark-800 dark:bg-dark-900"
        >
          <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="text-lg font-semibold text-slate-950 dark:text-white">{{ group.name }}</h2>
                <span class="rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600 dark:bg-dark-800 dark:text-slate-300">{{ platformLabel(group.platform) }}</span>
                <span v-if="group.is_exclusive" class="rounded-md bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-200">{{ localText('专属展示', 'Exclusive') }}</span>
                <span v-if="group.subscription_type === 'subscription'" class="rounded-md bg-sky-100 px-2 py-1 text-xs font-medium text-sky-700 dark:bg-sky-900/30 dark:text-sky-200">{{ localText('订阅分组', 'Subscription') }}</span>
              </div>
              <p v-if="group.description" class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">{{ group.description }}</p>
            </div>
            <div class="shrink-0 text-sm text-slate-500 dark:text-slate-400">
              {{ group.models.length }} {{ localText('个模型', 'models') }}
            </div>
          </div>

          <div class="mt-4 flex flex-wrap gap-2">
            <span
              v-for="model in group.models"
              :key="model"
              class="rounded-md border border-slate-200 bg-slate-50 px-2.5 py-1 font-mono text-xs text-slate-700 dark:border-dark-700 dark:bg-dark-950 dark:text-slate-200"
            >
              {{ model }}
            </span>
            <span v-if="group.models.length === 0" class="text-sm text-slate-500 dark:text-slate-400">
              {{ localText('暂无可展示模型', 'No visible models') }}
            </span>
          </div>

          <div class="mt-5 grid gap-3 lg:grid-cols-2">
            <div
              v-for="format in group.request_formats"
              :key="`${group.id}-${format.method}-${format.path}`"
              class="overflow-hidden rounded-lg border border-slate-200 dark:border-dark-700"
            >
              <div class="flex items-center justify-between gap-3 border-b border-slate-200 bg-slate-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-950">
                <div class="min-w-0">
                  <div class="truncate text-sm font-medium">{{ format.name }}</div>
                  <div class="mt-0.5 font-mono text-xs text-slate-500 dark:text-slate-400">
                    {{ format.method }} {{ format.path }}
                  </div>
                </div>
                <button
                  class="rounded-md p-2 text-slate-500 hover:bg-slate-200 hover:text-slate-800 dark:hover:bg-dark-800 dark:hover:text-white"
                  :title="localText('复制示例', 'Copy example')"
                  @click="copyFormat(format, group.models[0])"
                >
                  <Icon name="copy" size="sm" />
                </button>
              </div>
              <pre class="max-h-72 overflow-auto bg-slate-950 p-3 text-xs leading-5 text-slate-100"><code>{{ formatExample(format, group.models[0]) }}</code></pre>
            </div>
          </div>
        </article>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { modelMarketplaceAPI } from '@/api'
import type { ModelMarketplaceRequestFormat, ModelMarketplaceResponse } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'

const { locale } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const loading = ref(false)
const error = ref('')
const marketplace = ref<ModelMarketplaceResponse | null>(null)
const query = ref('')
const selectedPlatform = ref('all')

const isZhLocale = computed(() => locale.value.startsWith('zh'))
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')

const visibleGroups = computed(() => marketplace.value?.groups ?? [])
const platforms = computed(() => Array.from(new Set(visibleGroups.value.map((group) => group.platform))).sort())
const totalModels = computed(() => visibleGroups.value.reduce((sum, group) => sum + group.models.length, 0))
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
