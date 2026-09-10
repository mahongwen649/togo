<template>
  <AuthLayout :show-feature-summary="false">
    <div class="space-y-4 text-center">
      <h2 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('auth.forgotPasswordTitle') }}</h2>
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('auth.passwordResetUsesCode') }}</p>
      <router-link to="/forgot-password" class="btn btn-primary inline-flex items-center gap-2">
        <Icon name="mail" size="md" />
        {{ t('auth.sendVerificationCode') }}
      </router-link>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
</script>
