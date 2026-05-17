<template>
  <div class="mecha-app-shell min-h-screen bg-[#edf3fa] text-slate-900 dark:bg-[#03070d] dark:text-slate-100">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>
    <div class="pointer-events-none fixed inset-0 opacity-90 dark:opacity-100">
      <div class="hud-grid absolute inset-0"></div>
      <div class="absolute inset-0 bg-[linear-gradient(115deg,transparent_0%,transparent_42%,rgba(23,152,242,0.12)_42.2%,transparent_43.1%,transparent_72%,rgba(255,111,56,0.09)_72.2%,transparent_73%)]"></div>
      <div class="absolute right-0 top-0 h-40 w-[52vw] bg-[linear-gradient(135deg,rgba(255,255,255,0.74),rgba(75,181,255,0.16),transparent)] dark:bg-[linear-gradient(135deg,rgba(75,181,255,0.1),transparent)]"></div>
      <div class="absolute bottom-8 left-0 h-px w-[68vw] bg-gradient-to-r from-transparent via-primary-300/70 to-transparent"></div>
      <div class="absolute right-6 top-24 hidden h-28 w-28 border border-primary-300/35 [clip-path:polygon(18%_0,100%_0,100%_72%,72%_100%,0_100%,0_18%)] dark:border-primary-300/20 xl:block"></div>
      <div class="mecha-target-ring absolute right-[9vw] top-[18vh] hidden h-72 w-72 xl:block"></div>
      <div class="mecha-diagonal-beam absolute left-[18vw] top-0 hidden h-full w-44 lg:block"></div>
    </div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="relative p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
