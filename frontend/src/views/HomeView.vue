<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="homeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    ref="rootEl"
    class="relative flex min-h-screen flex-col overflow-hidden bg-gradient-to-br from-gray-50 via-primary-50/30 to-gray-100 dark:from-dark-950 dark:via-dark-900 dark:to-dark-950"
  >
    <!-- Animated Background -->
    <BackgroundFX variant="home" />

    <!-- Static decoration grid (kept for fallback) -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(20,184,166,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(20,184,166,0.03)_1px,transparent_1px)] bg-[size:64px_64px]"
      ></div>
    </div>

    <!-- Header -->
    <header class="relative z-20 px-6 py-4">
      <nav class="mx-auto flex max-w-6xl items-center justify-between">
        <!-- Logo + status pill -->
        <div class="flex items-center gap-3">
          <div class="h-10 w-10 overflow-hidden rounded-xl shadow-md live-glow">
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <div class="hidden flex-col leading-tight sm:flex">
            <span class="font-mono text-[10px] font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
              {{ t('home.live.coreOnline') }}
            </span>
            <span class="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
              <PulseDot tone="success" :title="t('home.live.systemNominal')" />
              <span>{{ t('home.live.systemNominal') }}</span>
            </span>
          </div>
        </div>

        <!-- Nav Actions -->
        <div class="flex items-center gap-3">
          <LocaleSwitcher />

          <router-link
            v-if="modelMarketplaceEnabled"
            to="/models"
            class="hidden items-center gap-2 rounded-full border border-primary-200/70 bg-white/80 px-3 py-1.5 text-xs font-semibold text-primary-700 shadow-sm backdrop-blur transition hover:border-primary-400 hover:bg-white dark:border-primary-400/25 dark:bg-dark-800/80 dark:text-primary-200 dark:hover:bg-dark-700 sm:inline-flex"
            :title="t('home.modelMarketplace')"
          >
            <Icon name="cube" size="sm" />
            <span>{{ t('home.modelMarketplace') }}</span>
          </router-link>

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <button
            @click="toggleTheme"
            class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-gray-900 py-1 pl-1 pr-2.5 transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-600 text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium text-white">{{ t('home.dashboard') }}</span>
            <svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 19.5l15-15m0 0H8.25m11.25 0v11.25" />
            </svg>
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-gray-900 px-3 py-1 text-xs font-medium text-white transition-colors hover:bg-gray-800 dark:bg-gray-800 dark:hover:bg-gray-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <!-- Main Content -->
    <main class="relative z-10 flex-1 px-6 py-12 md:py-16">
      <div class="mx-auto max-w-6xl">
        <!-- Hero Section -->
        <div class="mb-10 flex flex-col items-center justify-between gap-12 lg:flex-row lg:gap-16">
          <!-- Left: Text Content -->
          <div class="flex-1 text-center lg:text-left">
            <div class="hero-kicker mb-4 inline-flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.2em] text-primary-600 dark:text-primary-300">
              <PulseDot tone="primary" />
              <span>AI · API · GATEWAY</span>
            </div>

            <h1
              class="hero-title mb-4 text-4xl font-bold text-gray-900 dark:text-white md:text-5xl lg:text-6xl"
            >
              <span v-for="(word, idx) in titleWords" :key="idx" class="hero-word" :style="{ animationDelay: `${idx * 80}ms` }">
                {{ word }}<span v-if="idx < titleWords.length - 1">&nbsp;</span>
              </span>
            </h1>

            <p class="hero-subtitle mb-8 text-lg text-gray-600 dark:text-dark-300 md:text-xl">
              {{ siteSubtitle }}
              <span class="term-cursor align-middle text-primary-500"></span>
            </p>

            <!-- CTA Button -->
            <div class="hero-cta flex flex-col items-center gap-3 sm:flex-row lg:justify-start">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-primary px-8 py-3 text-base shadow-lg shadow-primary-500/30"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" class="ml-2" :stroke-width="2" />
              </router-link>
              <router-link
                v-if="modelMarketplaceEnabled"
                to="/models"
                class="btn btn-secondary px-8 py-3 text-base shadow-lg shadow-slate-900/5 dark:shadow-primary-500/10"
              >
                <Icon name="cube" size="md" :stroke-width="2" />
                {{ t('home.modelMarketplace') }}
              </router-link>
            </div>
          </div>

          <!-- Right: Terminal Animation -->
          <div class="flex flex-1 justify-center lg:justify-end">
            <div class="terminal-container tilt-surface" data-reveal>
              <div ref="terminalEl" class="terminal-window tilt-glow scan-host">
                <!-- Top scanline sweep -->
                <ScanlineSweep :duration="6" />

                <!-- Window header -->
                <div class="terminal-header">
                  <div class="terminal-buttons">
                    <span class="btn-close"></span>
                    <span class="btn-minimize"></span>
                    <span class="btn-maximize"></span>
                  </div>
                  <span class="terminal-title font-mono">{{ t('home.terminal.title') }}</span>
                  <span class="terminal-live flex items-center gap-1.5">
                    <PulseDot tone="success" />
                    <span class="font-mono text-[10px] uppercase tracking-widest">{{ t('home.live.label') }}</span>
                  </span>
                </div>

                <!-- Terminal content -->
                <div class="terminal-body">
                  <transition-group name="term-line" tag="div">
                    <div v-for="line in visibleLogs" :key="line.id" class="code-line">
                      <span v-if="line.kind === 'cmd'">
                        <span class="code-prompt">$</span>
                        <span class="code-cmd">curl</span>
                        <span class="code-flag">-X POST</span>
                        <span class="code-url">{{ line.text }}</span>
                      </span>
                      <span v-else-if="line.kind === 'comment'" class="code-comment">{{ line.text }}</span>
                      <span v-else-if="line.kind === 'success'" class="flex flex-wrap items-center gap-2">
                        <span class="code-success">200 OK</span>
                        <span class="code-response">{{ line.text }}</span>
                      </span>
                      <span v-else-if="line.kind === 'meta'" class="code-meta">{{ line.text }}</span>
                    </div>
                  </transition-group>
                  <div class="code-line">
                    <span class="code-prompt">$</span>
                    <span class="cursor"></span>
                  </div>
                </div>

                <!-- Footer status -->
                <div class="terminal-footer">
                  <span class="font-mono text-[10px] uppercase tracking-widest text-emerald-400">{{ t('home.terminal.streamHint') }}</span>
                  <span class="font-mono text-[10px] uppercase tracking-widest text-slate-500">{{ liveLatency }}ms</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Live Telemetry Strip -->
        <div class="live-strip mb-12" data-reveal data-reveal-delay="120">
          <div class="live-strip-grid grid grid-cols-2 gap-3 md:grid-cols-4">
            <div
              v-for="(metric, i) in liveMetrics"
              :key="metric.key"
              class="live-cell scan-host tilt-surface"
              :class="['live-cell-' + (i + 1)]"
            >
              <div class="flex items-center gap-2">
                <PulseDot :tone="metric.tone" />
                <span class="font-mono text-[10px] uppercase tracking-[0.2em] text-slate-500 dark:text-slate-400">{{ metric.label }}</span>
              </div>
              <div class="mt-2 flex items-baseline gap-1">
                <CountUp
                  class="live-cell-value"
                  :value="metric.value"
                  :decimals="metric.decimals"
                  :duration="1100"
                  :prefix="metric.prefix"
                  :suffix="metric.suffix"
                />
              </div>
              <div class="mt-1 font-mono text-[10px] uppercase text-slate-400 dark:text-slate-500">{{ metric.hint }}</div>
            </div>
          </div>
        </div>

        <!-- Feature Tags -->
        <div class="mb-10 flex flex-wrap items-center justify-center gap-4 md:gap-6" data-reveal>
          <div
            class="feature-chip inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <Icon name="swap" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.subscriptionToApi') }}</span>
          </div>
          <div
            class="feature-chip inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <Icon name="shield" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.stickySession') }}</span>
          </div>
          <div
            class="feature-chip inline-flex items-center gap-2.5 rounded-full border border-gray-200/50 bg-white/80 px-5 py-2.5 shadow-sm backdrop-blur-sm dark:border-dark-700/50 dark:bg-dark-800/80"
          >
            <Icon name="chart" size="sm" class="text-primary-500" />
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ t('home.tags.realtimeBilling') }}</span>
          </div>
        </div>

        <!-- Features Grid -->
        <div class="mb-12 grid gap-6 md:grid-cols-3">
          <div
            class="feature-card group rounded-2xl border border-gray-200/50 bg-white/60 p-6 backdrop-blur-sm transition-all duration-300 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
            data-reveal
            data-reveal-delay="0"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg shadow-blue-500/30 transition-transform group-hover:scale-110"
            >
              <Icon name="server" size="lg" class="text-white" />
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">{{ t('home.features.unifiedGateway') }}</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">{{ t('home.features.unifiedGatewayDesc') }}</p>
          </div>

          <div
            class="feature-card group rounded-2xl border border-gray-200/50 bg-white/60 p-6 backdrop-blur-sm transition-all duration-300 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
            data-reveal
            data-reveal-delay="80"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-primary-500 to-primary-600 shadow-lg shadow-primary-500/30 transition-transform group-hover:scale-110"
            >
              <svg class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M18 18.72a9.094 9.094 0 003.741-.479 3 3 0 00-4.682-2.72m.94 3.198l.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0112 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 016 18.719m12 0a5.971 5.971 0 00-.941-3.197m0 0A5.995 5.995 0 0012 12.75a5.995 5.995 0 00-5.058 2.772m0 0a3 3 0 00-4.681 2.72 8.986 8.986 0 003.74.477m.94-3.197a5.971 5.971 0 00-.94 3.197M15 6.75a3 3 0 11-6 0 3 3 0 016 0zm6 3a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0zm-13.5 0a2.25 2.25 0 11-4.5 0 2.25 2.25 0 014.5 0z" />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">{{ t('home.features.multiAccount') }}</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">{{ t('home.features.multiAccountDesc') }}</p>
          </div>

          <div
            class="feature-card group rounded-2xl border border-gray-200/50 bg-white/60 p-6 backdrop-blur-sm transition-all duration-300 hover:shadow-xl hover:shadow-primary-500/10 dark:border-dark-700/50 dark:bg-dark-800/60"
            data-reveal
            data-reveal-delay="160"
          >
            <div
              class="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-purple-500 to-purple-600 shadow-lg shadow-purple-500/30 transition-transform group-hover:scale-110"
            >
              <svg class="h-6 w-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 18.75a60.07 60.07 0 0115.797 2.101c.727.198 1.453-.342 1.453-1.096V18.75M3.75 4.5v.75A.75.75 0 013 6h-.75m0 0v-.375c0-.621.504-1.125 1.125-1.125H20.25M2.25 6v9m18-10.5v.75c0 .414.336.75.75.75h.75m-1.5-1.5h.375c.621 0 1.125.504 1.125 1.125v9.75c0 .621-.504 1.125-1.125 1.125h-.375m1.5-1.5H21a.75.75 0 00-.75.75v.75m0 0H3.75m0 0h-.375a1.125 1.125 0 01-1.125-1.125V15m1.5 1.5v-.75A.75.75 0 003 15h-.75M15 10.5a3 3 0 11-6 0 3 3 0 016 0zm3 0h.008v.008H18V10.5zm-12 0h.008v.008H6V10.5z" />
              </svg>
            </div>
            <h3 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">{{ t('home.features.balanceQuota') }}</h3>
            <p class="text-sm leading-relaxed text-gray-600 dark:text-dark-400">{{ t('home.features.balanceQuotaDesc') }}</p>
          </div>
        </div>

        <!-- Supported Providers -->
        <div class="mb-8 text-center" data-reveal>
          <h2 class="mb-3 text-2xl font-bold text-gray-900 dark:text-white">{{ t('home.providers.title') }}</h2>
          <p class="text-sm text-gray-600 dark:text-dark-400">{{ t('home.providers.description') }}</p>
        </div>

        <div class="mb-16 flex flex-wrap items-center justify-center gap-4">
          <div
            v-for="(p, i) in providerChips"
            :key="p.label"
            class="provider-chip flex items-center gap-2 rounded-xl border bg-white/60 px-5 py-3 backdrop-blur-sm dark:bg-dark-800/60"
            :class="p.supported ? 'border-primary-200 ring-1 ring-primary-500/20 dark:border-primary-800' : 'border-gray-200/50 opacity-60 dark:border-dark-700/50'"
            data-reveal
            :data-reveal-delay="i * 70"
          >
            <div
              class="flex h-8 w-8 items-center justify-center rounded-lg bg-gradient-to-br"
              :class="p.bg"
            >
              <span class="text-xs font-bold text-white">{{ p.initial }}</span>
            </div>
            <span class="text-sm font-medium text-gray-700 dark:text-dark-200">{{ p.label }}</span>
            <span
              class="rounded px-1.5 py-0.5 text-[10px] font-medium"
              :class="p.supported
                ? 'bg-primary-100 text-primary-600 dark:bg-primary-900/30 dark:text-primary-400'
                : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'"
            >
              {{ p.supported ? t('home.providers.supported') : t('home.providers.soon') }}
            </span>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-gray-200/50 px-6 py-8 dark:border-dark-800/50">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-center gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-gray-500 dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex items-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <a
            :href="githubUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-gray-500 transition-colors hover:text-gray-700 dark:text-dark-400 dark:hover:text-white"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import BackgroundFX from '@/components/common/BackgroundFX.vue'
import PulseDot from '@/components/common/PulseDot.vue'
import CountUp from '@/components/common/CountUp.vue'
import ScanlineSweep from '@/components/common/ScanlineSweep.vue'
import { useTilt } from '@/composables/useTilt'
import { useScrollReveal } from '@/composables/useScrollReveal'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const modelMarketplaceEnabled = computed(() => appStore.cachedPublicSettings?.model_marketplace_enabled !== false)

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const isDark = ref(document.documentElement.classList.contains('dark'))
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})
const currentYear = computed(() => new Date().getFullYear())

// Hero title word-split for staggered animation
const titleWords = computed(() => {
  const name = siteName.value || 'Sub2API'
  return name.split(/\s+/).filter(Boolean)
})

// Terminal log rotation
type LogEntry = { id: number; kind: 'cmd' | 'comment' | 'success' | 'meta'; text: string }

const logSequences = computed<LogEntry[][]>(() => {
  let counter = 0
  const next = () => ++counter
  return [
    [
      { id: next(), kind: 'cmd', text: '/v1/messages' },
      { id: next(), kind: 'comment', text: t('home.terminal.logRouting') },
      { id: next(), kind: 'success', text: t('home.terminal.response') },
      { id: next(), kind: 'meta', text: t('home.terminal.logCost') }
    ],
    [
      { id: next(), kind: 'cmd', text: '/v1/chat/completions' },
      { id: next(), kind: 'comment', text: t('home.terminal.logHandshake') },
      { id: next(), kind: 'success', text: t('home.terminal.logSticky') },
      { id: next(), kind: 'meta', text: t('home.terminal.logStream') }
    ],
    [
      { id: next(), kind: 'cmd', text: '/v1/responses' },
      { id: next(), kind: 'comment', text: t('home.terminal.logRouting') },
      { id: next(), kind: 'success', text: t('home.terminal.logHit') },
      { id: next(), kind: 'meta', text: t('home.terminal.logStream') }
    ]
  ]
})

const visibleLogs = ref<LogEntry[]>([])
let logIndex = 0
let logTimer: ReturnType<typeof setInterval> | null = null
let lineTimer: ReturnType<typeof setInterval> | null = null

function loadCurrentSequence() {
  const seq = logSequences.value[logIndex % logSequences.value.length]
  visibleLogs.value = []
  if (lineTimer) clearInterval(lineTimer)
  // Push first line immediately so terminal isn't empty on first paint
  let i = 0
  if (seq.length > 0) {
    visibleLogs.value = [seq[i]]
    i += 1
  }
  lineTimer = setInterval(() => {
    if (i >= seq.length) {
      if (lineTimer) clearInterval(lineTimer)
      lineTimer = null
      return
    }
    visibleLogs.value = [...visibleLogs.value, seq[i]]
    i += 1
  }, 700)
}

// Live metrics (mock + jitter)
type Tone = 'primary' | 'success' | 'warning'
interface Metric {
  key: string
  label: string
  value: number
  decimals: number
  prefix: string
  suffix: string
  hint: string
  tone: Tone
}

const liveMetrics = ref<Metric[]>([
  { key: 'requests', label: '', value: 1284763, decimals: 0, prefix: '', suffix: '', hint: '24h · auto-routed', tone: 'primary' },
  { key: 'channels', label: '', value: 42, decimals: 0, prefix: '', suffix: '', hint: 'multi-pool · sticky', tone: 'success' },
  { key: 'latency', label: '', value: 168, decimals: 0, prefix: '', suffix: ' ms', hint: 'p50 across regions', tone: 'warning' },
  { key: 'models', label: '', value: 27, decimals: 0, prefix: '', suffix: '', hint: 'claude · gpt · gemini', tone: 'primary' }
])

const liveLatency = ref(168)

function refreshLabels() {
  liveMetrics.value[0].label = t('home.live.requestsRouted')
  liveMetrics.value[1].label = t('home.live.activeChannels')
  liveMetrics.value[2].label = t('home.live.avgLatency')
  liveMetrics.value[3].label = t('home.live.modelsCovered')
}

let metricsTimer: ReturnType<typeof setInterval> | null = null
function jitterMetrics() {
  const m = liveMetrics.value
  m[0].value += Math.floor(20 + Math.random() * 80)
  m[1].value = 38 + Math.floor(Math.random() * 8)
  const newLatency = 145 + Math.floor(Math.random() * 60)
  m[2].value = newLatency
  liveLatency.value = newLatency
  m[3].value = 25 + Math.floor(Math.random() * 5)
}

// Provider chips
const providerChips = computed(() => [
  { label: t('home.providers.claude'), initial: 'C', bg: 'from-orange-400 to-orange-500', supported: true },
  { label: 'GPT', initial: 'G', bg: 'from-green-500 to-green-600', supported: true },
  { label: t('home.providers.gemini'), initial: 'G', bg: 'from-blue-500 to-blue-600', supported: true },
  { label: t('home.providers.antigravity'), initial: 'A', bg: 'from-rose-500 to-pink-600', supported: true },
  { label: t('home.providers.more'), initial: '+', bg: 'from-gray-500 to-gray-600', supported: false }
])

// Theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (savedTheme === 'dark') {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

// Tilt + scroll reveal
const rootEl = ref<HTMLElement | null>(null)
const terminalEl = ref<HTMLElement | null>(null)
useTilt(terminalEl, { max: 6, lift: 6, spotlight: true })
useScrollReveal(rootEl, { selector: '[data-reveal]', stagger: 70 })

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // Apply reveal-up to elements that have data-reveal
  if (rootEl.value) {
    rootEl.value.querySelectorAll<HTMLElement>('[data-reveal]').forEach((el) => {
      el.classList.add('reveal-up')
    })
  }

  refreshLabels()
  loadCurrentSequence()
  logTimer = setInterval(() => {
    logIndex += 1
    loadCurrentSequence()
  }, 6500)
  metricsTimer = setInterval(jitterMetrics, 3500)
})

onBeforeUnmount(() => {
  if (logTimer) clearInterval(logTimer)
  if (lineTimer) clearInterval(lineTimer)
  if (metricsTimer) clearInterval(metricsTimer)
})
</script>

<style scoped>
/* ====== Hero text reveal ====== */
.hero-kicker {
  opacity: 0;
  animation: hero-rise 540ms ease-out 0ms both;
}

.hero-title {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
}

@media (min-width: 1024px) {
  .hero-title {
    justify-content: flex-start;
  }
}

.hero-word {
  display: inline-block;
  opacity: 0;
  transform: translateY(28px) skewY(2deg);
  animation: hero-word-rise 720ms cubic-bezier(0.22, 1, 0.36, 1) forwards;
}

.hero-subtitle {
  opacity: 0;
  animation: hero-rise 600ms ease-out 480ms both;
}

.hero-cta {
  opacity: 0;
  animation: hero-rise 600ms ease-out 720ms both;
}

@keyframes hero-rise {
  from { opacity: 0; transform: translateY(14px); }
  to { opacity: 1; transform: translateY(0); }
}

@keyframes hero-word-rise {
  from { opacity: 0; transform: translateY(28px) skewY(2deg); }
  to { opacity: 1; transform: translateY(0) skewY(0); }
}

/* ====== Live strip ====== */
.live-cell {
  position: relative;
  padding: 0.85rem 1rem;
  border: 1px solid rgba(75, 181, 255, 0.3);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(232, 245, 255, 0.86)),
    linear-gradient(135deg, transparent 0 76%, rgba(255, 111, 56, 0.12) 76% 100%);
  clip-path: polygon(10px 0, 100% 0, 100% calc(100% - 12px), calc(100% - 12px) 100%, 0 100%, 0 10px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    0 12px 30px rgba(15, 68, 112, 0.1);
  overflow: hidden;
  transition: transform 280ms ease, box-shadow 280ms ease;
}

:global(.dark) .live-cell {
  border-color: rgba(75, 181, 255, 0.34);
  background:
    linear-gradient(135deg, rgba(7, 16, 28, 0.94), rgba(4, 10, 18, 0.82)),
    linear-gradient(135deg, transparent 0 76%, rgba(255, 111, 56, 0.12) 76% 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.08),
    0 18px 40px rgba(0, 0, 0, 0.4);
}

.live-cell:hover {
  transform: translateY(-2px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    0 18px 40px rgba(15, 68, 112, 0.16);
}

.live-cell::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, rgba(75, 181, 255, 0.95), rgba(255, 111, 56, 0.7), transparent);
  opacity: 0.85;
}

.live-cell-value {
  font-family: Bahnschrift, 'DIN Alternate', 'Arial Narrow', system-ui, sans-serif;
  font-size: 1.6rem;
  font-weight: 800;
  letter-spacing: 0;
  color: #0a3a64;
}

:global(.dark) .live-cell-value {
  color: #d4ecff;
  text-shadow: 0 0 14px rgba(75, 181, 255, 0.42);
}

/* ====== Provider chip hover sweep ====== */
.provider-chip {
  position: relative;
  overflow: hidden;
  transition: transform 240ms ease, box-shadow 240ms ease;
}

.provider-chip:hover {
  transform: translateY(-1px);
  box-shadow: 0 14px 36px rgba(15, 68, 112, 0.18);
}

.provider-chip::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(
    100deg,
    transparent 30%,
    rgba(75, 181, 255, 0.18) 50%,
    transparent 70%
  );
  transform: translateX(-110%);
  transition: transform 600ms ease;
}

.provider-chip:hover::after {
  transform: translateX(110%);
}

/* ====== Terminal Container ====== */
.terminal-container {
  position: relative;
  display: inline-block;
}

.terminal-window {
  width: 460px;
  max-width: 92vw;
  background: linear-gradient(145deg, #1e293b 0%, #0f172a 100%);
  border-radius: 14px;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.45),
    0 0 0 1px rgba(75, 181, 255, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  overflow: hidden;
  transition: transform 0.3s ease, box-shadow 0.3s ease;
  position: relative;
  isolation: isolate;
}

/* Header */
.terminal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(30, 41, 59, 0.85);
  border-bottom: 1px solid rgba(75, 181, 255, 0.16);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 12px;
  height: 12px;
  border-radius: 50%;
}

.btn-close { background: #ef4444; }
.btn-minimize { background: #eab308; }
.btn-maximize { background: #22c55e; }

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  color: #94a3b8;
}

.terminal-live {
  color: #34d399;
}

/* Body */
.terminal-body {
  padding: 18px 22px 14px;
  font-family: ui-monospace, 'Fira Code', monospace;
  font-size: 13.5px;
  line-height: 2;
  min-height: 168px;
  position: relative;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.code-prompt { color: #22c55e; font-weight: bold; }
.code-cmd { color: #38bdf8; }
.code-flag { color: #a78bfa; }
.code-url { color: #14b8a6; }
.code-comment { color: #64748b; font-style: italic; }
.code-success {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.15);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response { color: #fbbf24; }
.code-meta { color: #94a3b8; }

/* Term-line transition */
.term-line-enter-active,
.term-line-leave-active {
  transition: opacity 320ms ease, transform 320ms ease;
}
.term-line-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.term-line-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

/* Footer */
.terminal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 16px;
  border-top: 1px solid rgba(75, 181, 255, 0.16);
  background: rgba(11, 20, 32, 0.85);
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: #22c55e;
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* Dark mode tweaks */
:global(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(75, 181, 255, 0.32),
    0 0 60px rgba(75, 181, 255, 0.18),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

@media (max-width: 768px) {
  .terminal-window {
    width: 100%;
  }
  .live-cell-value { font-size: 1.35rem; }
}

@media (prefers-reduced-motion: reduce) {
  .hero-kicker,
  .hero-word,
  .hero-subtitle,
  .hero-cta {
    opacity: 1 !important;
    animation: none !important;
    transform: none !important;
  }
}
</style>
