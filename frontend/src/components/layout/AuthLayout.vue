<template>
  <div class="relative flex min-h-screen items-center justify-center overflow-hidden bg-[#edf4fc] p-4 dark:bg-[#080d15]">
    <!-- Background -->
    <div
      class="absolute inset-0 bg-cover bg-center opacity-92 dark:opacity-55"
      style="background-image: url('/theme/staly.png')"
    ></div>
    <div class="absolute inset-0 bg-[linear-gradient(90deg,rgba(247,250,255,0.92)_0%,rgba(247,250,255,0.78)_38%,rgba(247,250,255,0.2)_78%,rgba(247,250,255,0.52)_100%)] dark:bg-[linear-gradient(90deg,rgba(8,13,21,0.94)_0%,rgba(8,13,21,0.78)_38%,rgba(8,13,21,0.22)_76%,rgba(8,13,21,0.7)_100%)]"></div>

    <!-- Decorative Elements -->
    <div class="pointer-events-none absolute inset-0 overflow-hidden">
      <div
        class="absolute inset-0 bg-[linear-gradient(rgba(23,152,242,0.075)_1px,transparent_1px),linear-gradient(90deg,rgba(23,152,242,0.055)_1px,transparent_1px)] bg-[size:52px_52px]"
      ></div>
      <div class="absolute left-[8%] top-0 h-full w-px bg-gradient-to-b from-transparent via-primary-400/40 to-transparent"></div>
      <div class="absolute bottom-12 left-0 h-px w-2/3 bg-gradient-to-r from-transparent via-primary-400/40 to-transparent"></div>
      <div class="absolute right-10 top-10 h-28 w-28 border border-primary-300/30 [clip-path:polygon(18%_0,100%_0,100%_72%,72%_100%,0_100%,0_18%)]"></div>
    </div>

    <!-- Content Container -->
    <div class="relative z-10 w-full max-w-md">
      <!-- Logo/Brand -->
      <div class="mb-8 text-center">
        <!-- Custom Logo or Default Logo -->
        <template v-if="settingsLoaded">
          <div
            class="mb-4 inline-flex h-16 w-16 items-center justify-center overflow-hidden rounded-lg border border-primary-200/95 bg-white/95 shadow-lg shadow-primary-500/25 backdrop-blur-xl dark:border-primary-400/30 dark:bg-[#0f1724]/75"
          >
            <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
          </div>
          <h1 class="text-gradient mb-2 text-3xl font-bold">
            {{ siteName }}
          </h1>
          <p class="text-sm font-medium text-slate-500 dark:text-slate-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <!-- Card Container -->
      <div class="card-glass mecha-panel rounded-lg p-8 shadow-glass">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-slate-500 dark:text-slate-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
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
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.text-gradient {
  @apply bg-gradient-to-r from-primary-700 via-primary-500 to-orange-500 bg-clip-text text-transparent dark:from-primary-200 dark:via-primary-400 dark:to-orange-300;
}
</style>
