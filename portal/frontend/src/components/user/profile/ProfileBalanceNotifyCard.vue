<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.balanceNotify.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('profile.balanceNotify.description') }}
      </p>
    </div>
    <div class="px-6 py-6">
      <div class="flex items-center justify-between">
        <label class="input-label mb-0">{{ t('profile.balanceNotify.enabled') }}</label>
        <label class="relative inline-flex cursor-pointer items-center">
          <input
            v-model="notifyEnabled"
            type="checkbox"
            class="peer sr-only"
            @change="handleToggle"
          />
          <div class="h-6 w-11 rounded-full bg-gray-200 after:absolute after:left-[2px] after:top-[2px] after:h-5 after:w-5 after:rounded-full after:border after:border-gray-300 after:bg-white after:transition-all after:content-[''] peer-checked:bg-primary-600 peer-checked:after:translate-x-full peer-checked:after:border-white peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-primary-300 dark:bg-gray-700 dark:after:border-gray-600 dark:peer-focus:ring-primary-800"></div>
        </label>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { userAPI } from '@/api/user'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  enabled: boolean
}>()

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const notifyEnabled = ref(props.enabled)

watch(() => props.enabled, (value) => {
  notifyEnabled.value = value
})

async function handleToggle() {
  try {
    const updated = await userAPI.updateProfile({ balance_notify_enabled: notifyEnabled.value })
    authStore.user = updated
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
    notifyEnabled.value = !notifyEnabled.value
  }
}
</script>
