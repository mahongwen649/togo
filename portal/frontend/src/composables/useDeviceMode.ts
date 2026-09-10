import { computed, ref } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { isMobileDevice as detectCurrentDevice } from '@/utils/device'

const MOBILE_VIEWPORT_QUERY = '(max-width: 767px)'

function isH5PreviewEnabled(): boolean {
  if (!import.meta.env.DEV || typeof window === 'undefined') return false
  return new URLSearchParams(window.location.search).get('h5') === '1'
}

export function useDeviceMode() {
  const isMobileDevice = ref(detectCurrentDevice() || isH5PreviewEnabled())
  const isMobileViewport = useMediaQuery(MOBILE_VIEWPORT_QUERY)

  const isMobileH5 = computed(() => isMobileDevice.value)
  const isCompactNavigation = computed(() => isMobileDevice.value)
  const isDesktop = computed(() => !isMobileH5.value)

  return {
    isCompactNavigation,
    isDesktop,
    isMobileDevice,
    isMobileH5,
    isMobileViewport,
  }
}
