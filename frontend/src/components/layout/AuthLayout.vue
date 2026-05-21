<template>
  <div class="auth-shell relative min-h-screen overflow-hidden bg-[#eaf1f8] text-slate-950 dark:bg-[#03070d] dark:text-white">
    <div class="auth-visual absolute inset-0"></div>
    <div class="auth-overlay absolute inset-0"></div>

    <!-- Dynamic Background (canvas particles + drifting beams) -->
    <BackgroundFX variant="auth" :density="0.85" />

    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="hud-grid absolute inset-0"></div>
      <div class="scanline absolute inset-0"></div>
      <div class="absolute left-[7vw] top-0 hidden h-full w-px bg-gradient-to-b from-transparent via-primary-300/70 to-transparent md:block"></div>
      <div class="absolute bottom-[13vh] left-0 h-px w-[72vw] bg-gradient-to-r from-transparent via-primary-300/70 to-transparent"></div>
      <div class="armor-mark absolute right-[7vw] top-[8vh] hidden h-28 w-28 border border-primary-200/60 dark:border-primary-300/30 md:block"></div>
      <div class="armor-mark absolute bottom-[8vh] left-[12vw] h-20 w-44 border border-orange-300/40 dark:border-orange-300/25"></div>
      <div class="auth-mecha-reticle mecha-target-spin absolute right-[10vw] top-[18vh] hidden h-80 w-80 lg:block"></div>
      <div class="auth-energy-spine absolute bottom-0 right-[32vw] hidden h-[74vh] w-10 lg:block">
        <div class="auth-energy-spine-beam"></div>
      </div>
    </div>

    <div class="relative z-10 grid min-h-screen grid-cols-1 items-center px-4 py-8 md:px-10 lg:grid-cols-[minmax(420px,520px)_1fr] lg:py-10">
      <section class="auth-console relative w-full max-w-[520px] mecha-unlock">
        <div class="auth-console-rail"></div>

        <div class="relative p-5 sm:p-7">
          <template v-if="settingsLoaded">
            <div class="mb-7 flex items-center gap-4 auth-entrance">
              <div
                class="auth-logo live-glow flex h-14 w-14 items-center justify-center overflow-hidden border border-primary-200/90 bg-white/90 shadow-glow backdrop-blur-xl dark:border-primary-300/35 dark:bg-[#0b1420]/85"
              >
                <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
              </div>
              <div class="min-w-0">
                <div class="mb-1 flex items-center gap-2 font-mono text-[10px] font-semibold uppercase text-primary-600 dark:text-primary-300">
                  <PulseDot tone="primary" />
                  <span>ACCESS TERMINAL</span>
                </div>
                <h1 class="truncate text-3xl font-black leading-tight text-slate-950 dark:text-white">
                  {{ siteName }}
                </h1>
                <p class="mt-1 line-clamp-2 text-sm font-medium text-slate-600 dark:text-slate-300">
                  {{ siteSubtitle }}
                </p>
              </div>
            </div>
          </template>

          <div class="mecha-login-panel scan-host p-5 sm:p-6 auth-entrance auth-entrance-delay">
            <ScanlineSweep :duration="5" />
            <slot />
          </div>

          <div class="mt-5 text-center text-sm">
            <slot name="footer" />
          </div>

          <div class="mt-6 flex items-center justify-between gap-4 font-mono text-[10px] uppercase text-slate-500 dark:text-slate-500">
            <span class="flex items-center gap-1.5">
              <PulseDot tone="success" />
              <span>CORE ONLINE</span>
            </span>
            <span>&copy; {{ currentYear }} {{ siteName }}</span>
          </div>
        </div>
      </section>

      <section class="pointer-events-none hidden min-h-[620px] items-end justify-end lg:flex">
        <div class="auth-hud-panel mb-10 mr-8 w-[min(34vw,520px)]">
          <div class="mb-4 flex items-center justify-between font-mono text-[10px] uppercase text-primary-700 dark:text-primary-200">
            <span>NEURAL GATEWAY</span>
            <span class="inline-flex items-center gap-1.5">
              <span>SYNC</span>
              <CountUp :value="100" suffix="%" :duration="1400" />
            </span>
          </div>
          <div class="h-2 overflow-hidden bg-slate-900/10 dark:bg-white/10 relative">
            <div class="h-full w-[74%] bg-gradient-to-r from-primary-400 via-cyan-200 to-orange-300 auth-progress-bar"></div>
          </div>
          <div class="mt-5 grid grid-cols-3 gap-2 font-mono text-[10px] uppercase text-slate-600 dark:text-slate-300">
            <span class="auth-metric">AUTH</span>
            <span class="auth-metric">TOKEN</span>
            <span class="auth-metric">ROUTE</span>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import BackgroundFX from '@/components/common/BackgroundFX.vue'
import PulseDot from '@/components/common/PulseDot.vue'
import CountUp from '@/components/common/CountUp.vue'
import ScanlineSweep from '@/components/common/ScanlineSweep.vue'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  preloadAuthBackground()
  appStore.fetchPublicSettings()
})

function preloadAuthBackground() {
  if (typeof document === 'undefined') return
  const isMobile = window.matchMedia?.('(max-width: 768px)').matches
  const href = isMobile ? '/theme/staly-login-mobile.webp' : '/theme/staly-login.webp'
  if (document.querySelector(`link[rel="preload"][href="${href}"]`)) return
  const link = document.createElement('link')
  link.rel = 'preload'
  link.as = 'image'
  link.href = href
  document.head.appendChild(link)
}
</script>

<style scoped>
.auth-visual {
  background-image: image-set(url('/theme/staly-login.webp') type('image/webp'), url('/theme/staly.png') type('image/png'));
  background-position: center right 14%;
  background-size: cover;
  opacity: 0.98;
  filter: saturate(1.08) contrast(1.06) brightness(1.03);
}

.auth-overlay {
  background:
    linear-gradient(105deg, rgba(248, 252, 255, 0.99) 0%, rgba(232, 243, 253, 0.9) 32%, rgba(238, 246, 255, 0.18) 60%, rgba(236, 245, 253, 0.5) 100%),
    linear-gradient(118deg, transparent 0 43%, rgba(23, 152, 242, 0.12) 43.2% 43.55%, transparent 43.75% 100%),
    linear-gradient(92deg, transparent 0 76%, rgba(255, 111, 56, 0.14) 76.2% 76.5%, transparent 76.7% 100%),
    repeating-linear-gradient(0deg, transparent 0 12px, rgba(23, 152, 242, 0.035) 13px, transparent 14px);
}

:global(.dark .auth-overlay) {
  background:
    linear-gradient(105deg, rgba(2, 5, 10, 0.99) 0%, rgba(3, 8, 15, 0.9) 34%, rgba(4, 9, 18, 0.08) 66%, rgba(4, 9, 18, 0.62) 100%),
    radial-gradient(circle at 28% 20%, rgba(75, 181, 255, 0.28), transparent 34%),
    radial-gradient(circle at 82% 70%, rgba(255, 111, 56, 0.2), transparent 30%),
    repeating-linear-gradient(0deg, transparent 0 12px, rgba(75, 181, 255, 0.045) 13px, transparent 14px);
}

.hud-grid {
  background-image:
    linear-gradient(rgba(23, 152, 242, 0.11) 1px, transparent 1px),
    linear-gradient(90deg, rgba(23, 152, 242, 0.08) 1px, transparent 1px),
    linear-gradient(135deg, transparent 0 48%, rgba(255, 111, 56, 0.18) 48.2% 48.6%, transparent 48.8% 100%);
  background-size: 56px 56px, 56px 56px, 100% 100%;
}

.scanline {
  background: repeating-linear-gradient(0deg, transparent 0 9px, rgba(75, 181, 255, 0.06) 10px, transparent 11px);
  mix-blend-mode: multiply;
}

:global(.dark .scanline) {
  mix-blend-mode: screen;
}

.auth-mecha-reticle {
  border: 1px solid rgba(75, 181, 255, 0.24);
  clip-path: polygon(50% 0, 100% 18%, 100% 82%, 50% 100%, 0 82%, 0 18%);
  background:
    linear-gradient(90deg, transparent 49.6%, rgba(75, 181, 255, 0.3) 49.8% 50.2%, transparent 50.4%),
    linear-gradient(0deg, transparent 49.6%, rgba(75, 181, 255, 0.22) 49.8% 50.2%, transparent 50.4%),
    radial-gradient(circle, transparent 0 34%, rgba(75, 181, 255, 0.18) 34.4% 34.8%, transparent 35.2% 100%);
  box-shadow: inset 0 0 60px rgba(75, 181, 255, 0.12), 0 0 70px rgba(75, 181, 255, 0.08);
}

.auth-energy-spine {
  transform: skewX(-18deg);
  background: linear-gradient(180deg, transparent, rgba(75, 181, 255, 0.18), rgba(255, 111, 56, 0.12), transparent);
  border-left: 1px solid rgba(75, 181, 255, 0.18);
  border-right: 1px solid rgba(255, 111, 56, 0.16);
  filter: blur(0.1px);
}

.armor-mark,
.auth-console,
.auth-logo,
.mecha-login-panel,
.auth-hud-panel {
  clip-path: polygon(18px 0, 100% 0, 100% calc(100% - 22px), calc(100% - 22px) 100%, 0 100%, 0 18px);
}

.auth-console {
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(232, 244, 255, 0.84)),
    linear-gradient(90deg, rgba(23, 152, 242, 0.24), transparent 24%, transparent 78%, rgba(255, 111, 56, 0.16));
  border: 1px solid rgba(23, 152, 242, 0.36);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    inset 0 -1px 0 rgba(23, 152, 242, 0.12),
    0 30px 78px rgba(15, 68, 112, 0.2),
    0 0 0 1px rgba(255, 255, 255, 0.42);
  backdrop-filter: blur(22px);
}

:global(.dark .auth-console) {
  background:
    linear-gradient(135deg, rgba(7, 16, 28, 0.96), rgba(5, 12, 22, 0.76)),
    linear-gradient(90deg, rgba(75, 181, 255, 0.25), transparent 24%, transparent 78%, rgba(255, 111, 56, 0.18));
  border-color: rgba(75, 181, 255, 0.38);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    0 30px 90px rgba(0, 0, 0, 0.45),
    0 0 52px rgba(23, 152, 242, 0.2);
}

.auth-console-rail {
  position: absolute;
  left: 0;
  top: 24px;
  bottom: 24px;
  width: 4px;
  background: linear-gradient(180deg, transparent, #4bb5ff, #ff6f38, transparent);
  box-shadow: 0 0 24px rgba(75, 181, 255, 0.75);
  overflow: hidden;
}

.auth-console-rail::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: -40%;
  height: 30%;
  background: linear-gradient(180deg, transparent, rgba(255, 255, 255, 0.95), transparent);
  animation: auth-rail-pulse 3.6s ease-in-out infinite;
}

@keyframes auth-rail-pulse {
  0% { transform: translateY(0%); opacity: 0; }
  10% { opacity: 1; }
  90% { opacity: 1; }
  100% { transform: translateY(420%); opacity: 0; }
}

.auth-energy-spine-beam {
  position: absolute;
  left: 0;
  right: 0;
  top: -30%;
  height: 35%;
  background: linear-gradient(180deg, transparent, rgba(75, 181, 255, 0.85), rgba(255, 111, 56, 0.6), transparent);
  filter: blur(1px);
  animation: auth-spine-sweep 5.2s ease-in-out infinite;
}

@keyframes auth-spine-sweep {
  0% { transform: translateY(0%); opacity: 0.2; }
  20% { opacity: 0.9; }
  80% { opacity: 0.9; }
  100% { transform: translateY(360%); opacity: 0.2; }
}

.auth-progress-bar {
  position: relative;
  overflow: hidden;
}

.auth-progress-bar::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.78), transparent);
  transform: translateX(-100%);
  animation: auth-progress-shine 2.6s linear infinite;
}

@keyframes auth-progress-shine {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(220%); }
}

@media (prefers-reduced-motion: reduce) {
  .auth-console-rail::after,
  .auth-energy-spine-beam,
  .auth-progress-bar::after {
    animation: none !important;
  }
}

.mecha-login-panel {
  position: relative;
  border: 1px solid rgba(23, 152, 242, 0.32);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(238, 247, 255, 0.82)),
    linear-gradient(135deg, rgba(23, 152, 242, 0.18), transparent 36%, rgba(255, 111, 56, 0.12));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.82),
    0 12px 28px rgba(15, 68, 112, 0.1);
}

:global(.dark .mecha-login-panel) {
  background:
    linear-gradient(180deg, rgba(6, 15, 27, 0.92), rgba(4, 10, 18, 0.86)),
    linear-gradient(135deg, rgba(75, 181, 255, 0.18), transparent 36%, rgba(255, 111, 56, 0.12));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

.mecha-login-panel::before {
  content: '';
  position: absolute;
  left: 18px;
  right: 18px;
  top: 0;
  height: 2px;
  background: linear-gradient(90deg, transparent, rgba(75, 181, 255, 0.95), rgba(255, 111, 56, 0.75), transparent);
}

.auth-hud-panel {
  border: 1px solid rgba(23, 152, 242, 0.32);
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.76), rgba(226, 239, 251, 0.62)),
    linear-gradient(135deg, transparent 0 76%, rgba(255, 111, 56, 0.14) 76% 100%);
  padding: 18px;
  backdrop-filter: blur(12px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 18px 60px rgba(15, 68, 112, 0.2);
}

:global(.dark .auth-hud-panel) {
  background: rgba(5, 12, 22, 0.46);
}

.auth-metric {
  border: 1px solid rgba(23, 152, 242, 0.3);
  padding: 0.375rem 0.5rem;
  text-align: center;
  background:
    linear-gradient(135deg, rgba(223, 241, 255, 0.86), rgba(255, 255, 255, 0.62));
  clip-path: polygon(7px 0, 100% 0, 100% calc(100% - 7px), calc(100% - 7px) 100%, 0 100%, 0 7px);
}

.auth-entrance {
  animation: authRise 520ms ease-out both;
}

.auth-entrance-delay {
  animation-delay: 90ms;
}

.auth-form-title h2 {
  font-family: Bahnschrift, 'DIN Alternate', 'Arial Narrow', system-ui, sans-serif;
}

@keyframes authRise {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@media (max-width: 768px) {
  .auth-visual {
    background-image: image-set(url('/theme/staly-login-mobile.webp') type('image/webp'), url('/theme/staly.png') type('image/png'));
    background-position: center top;
    opacity: 0.88;
  }

  .auth-overlay {
    background:
      linear-gradient(180deg, rgba(238, 246, 255, 0.94) 0%, rgba(238, 246, 255, 0.86) 44%, rgba(238, 246, 255, 0.96) 100%),
      radial-gradient(circle at 50% 8%, rgba(75, 181, 255, 0.2), transparent 32%);
  }

  :global(.dark .auth-overlay) {
    background:
      linear-gradient(180deg, rgba(4, 9, 18, 0.92) 0%, rgba(4, 9, 18, 0.84) 44%, rgba(4, 9, 18, 0.96) 100%),
      radial-gradient(circle at 50% 8%, rgba(75, 181, 255, 0.2), transparent 32%);
  }
}
</style>
