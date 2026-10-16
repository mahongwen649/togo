<template>
  <div
    class="min-h-screen bg-[#f5faf9] dark:bg-dark-950"
    :class="[{ 'viewport-layout': viewport }, isMobileH5 ? 'overflow-x-clip' : '']"
  >
    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="layout-content relative min-w-0 min-h-screen transition-all duration-300"
      :class="isCompactNavigation ? 'ml-0' : sidebarCollapsed ? 'ml-[72px]' : 'ml-64'"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main :class="isMobileH5 ? 'p-2' : 'p-4 md:p-6 lg:p-8'">
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
import { useDeviceMode } from '@/composables/useDeviceMode'

defineProps<{ viewport?: boolean }>()

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const { isCompactNavigation, isMobileH5 } = useDeviceMode()
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

<style scoped>
.viewport-layout { height: 100dvh; min-height: 0; overflow: hidden; }
.viewport-layout > .layout-content { height: 100%; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
.viewport-layout > .layout-content > :deep(header) { flex-shrink: 0; }
.viewport-layout main { flex: 1; min-height: 0; min-width: 0; overflow: hidden; padding: 16px; }
@media (max-width: 700px) { .viewport-layout main { padding: 8px; } }
</style>
