<template>
  <div class="min-h-screen bg-[#f3f7fc] text-slate-900 dark:bg-[#080d15] dark:text-slate-100">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>
    <div class="pointer-events-none fixed inset-0 opacity-70 dark:opacity-45">
      <div class="absolute inset-0 bg-[linear-gradient(115deg,transparent_0%,transparent_48%,rgba(23,152,242,0.08)_48.2%,transparent_49.2%,transparent_72%,rgba(255,111,56,0.06)_72.2%,transparent_73%)]"></div>
      <div class="absolute right-0 top-0 h-32 w-[48vw] bg-[linear-gradient(135deg,rgba(255,255,255,0.72),rgba(75,181,255,0.12),transparent)] dark:bg-[linear-gradient(135deg,rgba(75,181,255,0.08),transparent)]"></div>
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
