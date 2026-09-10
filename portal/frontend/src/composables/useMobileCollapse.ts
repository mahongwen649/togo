import { computed, ref } from 'vue'
import { useDeviceMode } from './useDeviceMode'

export function useMobileCollapse(defaultCollapsed = true) {
  const { isMobileH5 } = useDeviceMode()
  const collapsed = ref(defaultCollapsed)
  const contentVisible = computed(() => !isMobileH5.value || !collapsed.value)

  const toggleMobile = () => {
    if (isMobileH5.value) collapsed.value = !collapsed.value
  }

  return {
    collapsed,
    contentVisible,
    isMobileH5,
    toggleMobile,
  }
}
