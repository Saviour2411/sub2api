<template>
  <div class="auth-shell relative min-h-screen overflow-hidden bg-[#eaf1f8] text-slate-950 dark:bg-[#040912] dark:text-white">
    <div class="auth-visual absolute inset-0"></div>
    <div class="auth-overlay absolute inset-0"></div>

    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div class="hud-grid absolute inset-0"></div>
      <div class="scanline absolute inset-0"></div>
      <div class="absolute left-[7vw] top-0 hidden h-full w-px bg-gradient-to-b from-transparent via-primary-300/70 to-transparent md:block"></div>
      <div class="absolute bottom-[13vh] left-0 h-px w-[72vw] bg-gradient-to-r from-transparent via-primary-300/70 to-transparent"></div>
      <div class="armor-mark absolute right-[7vw] top-[8vh] hidden h-28 w-28 border border-primary-200/60 dark:border-primary-300/30 md:block"></div>
      <div class="armor-mark absolute bottom-[8vh] left-[12vw] h-20 w-44 border border-orange-300/40 dark:border-orange-300/25"></div>
    </div>

    <div class="relative z-10 grid min-h-screen grid-cols-1 items-center px-4 py-8 md:px-10 lg:grid-cols-[minmax(420px,520px)_1fr] lg:py-10">
      <section class="auth-console relative w-full max-w-[520px]">
        <div class="auth-console-rail"></div>

        <div class="relative p-5 sm:p-7">
          <template v-if="settingsLoaded">
            <div class="mb-7 flex items-center gap-4">
              <div
                class="auth-logo flex h-14 w-14 items-center justify-center overflow-hidden border border-primary-200/90 bg-white/90 shadow-glow backdrop-blur-xl dark:border-primary-300/35 dark:bg-[#0b1420]/85"
              >
                <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
              </div>
              <div class="min-w-0">
                <div class="mb-1 font-mono text-[10px] font-semibold uppercase text-primary-600 dark:text-primary-300">
                  ACCESS TERMINAL
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

          <div class="mecha-login-panel p-5 sm:p-6">
            <slot />
          </div>

          <div class="mt-5 text-center text-sm">
            <slot name="footer" />
          </div>

          <div class="mt-6 flex items-center justify-between gap-4 font-mono text-[10px] uppercase text-slate-500 dark:text-slate-500">
            <span>CORE ONLINE</span>
            <span>&copy; {{ currentYear }} {{ siteName }}</span>
          </div>
        </div>
      </section>

      <section class="pointer-events-none hidden min-h-[620px] items-end justify-end lg:flex">
        <div class="auth-hud-panel mb-10 mr-8 w-[min(34vw,520px)]">
          <div class="mb-4 flex items-center justify-between font-mono text-[10px] uppercase text-primary-700 dark:text-primary-200">
            <span>NEURAL GATEWAY</span>
            <span>SYNC 100%</span>
          </div>
          <div class="h-2 overflow-hidden bg-slate-900/10 dark:bg-white/10">
            <div class="h-full w-[74%] bg-gradient-to-r from-primary-400 via-cyan-200 to-orange-300"></div>
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
  background-position: center right 18%;
  background-size: cover;
  opacity: 0.92;
}

.auth-overlay {
  background:
    linear-gradient(105deg, rgba(238, 246, 255, 0.98) 0%, rgba(238, 246, 255, 0.88) 35%, rgba(238, 246, 255, 0.12) 68%, rgba(238, 246, 255, 0.5) 100%),
    radial-gradient(circle at 28% 20%, rgba(75, 181, 255, 0.22), transparent 34%),
    radial-gradient(circle at 82% 70%, rgba(255, 111, 56, 0.16), transparent 30%);
}

:global(.dark) .auth-overlay {
  background:
    linear-gradient(105deg, rgba(4, 9, 18, 0.98) 0%, rgba(4, 9, 18, 0.86) 36%, rgba(4, 9, 18, 0.18) 68%, rgba(4, 9, 18, 0.7) 100%),
    radial-gradient(circle at 28% 20%, rgba(75, 181, 255, 0.22), transparent 34%),
    radial-gradient(circle at 82% 70%, rgba(255, 111, 56, 0.16), transparent 30%);
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
  mix-blend-mode: screen;
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
    linear-gradient(135deg, rgba(255, 255, 255, 0.9), rgba(226, 238, 251, 0.74)),
    linear-gradient(90deg, rgba(23, 152, 242, 0.2), transparent 24%, transparent 78%, rgba(255, 111, 56, 0.14));
  border: 1px solid rgba(75, 181, 255, 0.28);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.75),
    0 28px 80px rgba(8, 47, 88, 0.18),
    0 0 0 1px rgba(255, 255, 255, 0.36);
  backdrop-filter: blur(22px);
}

:global(.dark) .auth-console {
  background:
    linear-gradient(135deg, rgba(9, 17, 29, 0.92), rgba(13, 28, 44, 0.72)),
    linear-gradient(90deg, rgba(75, 181, 255, 0.2), transparent 24%, transparent 78%, rgba(255, 111, 56, 0.16));
  border-color: rgba(75, 181, 255, 0.3);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.12),
    0 30px 90px rgba(0, 0, 0, 0.45),
    0 0 42px rgba(23, 152, 242, 0.14);
}

.auth-console-rail {
  position: absolute;
  left: 0;
  top: 24px;
  bottom: 24px;
  width: 4px;
  background: linear-gradient(180deg, transparent, #4bb5ff, #ff6f38, transparent);
  box-shadow: 0 0 24px rgba(75, 181, 255, 0.75);
}

.mecha-login-panel {
  position: relative;
  border: 1px solid rgba(75, 181, 255, 0.26);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.86), rgba(239, 246, 255, 0.74)),
    linear-gradient(135deg, rgba(75, 181, 255, 0.16), transparent 36%, rgba(255, 111, 56, 0.1));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

:global(.dark) .mecha-login-panel {
  background:
    linear-gradient(180deg, rgba(8, 18, 31, 0.86), rgba(11, 22, 36, 0.78)),
    linear-gradient(135deg, rgba(75, 181, 255, 0.14), transparent 36%, rgba(255, 111, 56, 0.09));
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
  border: 1px solid rgba(75, 181, 255, 0.24);
  background: rgba(239, 246, 255, 0.56);
  padding: 18px;
  backdrop-filter: blur(12px);
  box-shadow: 0 18px 60px rgba(8, 47, 88, 0.18);
}

:global(.dark) .auth-hud-panel {
  background: rgba(5, 12, 22, 0.46);
}

@media (max-width: 768px) {
  .auth-visual {
    background-image: image-set(url('/theme/staly-login-mobile.webp') type('image/webp'), url('/theme/staly.png') type('image/png'));
    background-position: center top;
  }

  .auth-overlay {
    background:
      linear-gradient(180deg, rgba(238, 246, 255, 0.94) 0%, rgba(238, 246, 255, 0.86) 44%, rgba(238, 246, 255, 0.96) 100%),
      radial-gradient(circle at 50% 8%, rgba(75, 181, 255, 0.2), transparent 32%);
  }

  :global(.dark) .auth-overlay {
    background:
      linear-gradient(180deg, rgba(4, 9, 18, 0.92) 0%, rgba(4, 9, 18, 0.84) 44%, rgba(4, 9, 18, 0.96) 100%),
      radial-gradient(circle at 50% 8%, rgba(75, 181, 255, 0.2), transparent 32%);
  }
}
</style>
