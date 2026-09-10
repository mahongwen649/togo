<template>
  <AuthLayout :show-brand-panel="false">
    <div class="space-y-6">
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('auth.forgotPasswordTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('auth.forgotPasswordHint') }}</p>
      </div>

      <form class="space-y-5" @submit.prevent="handleSubmit">
        <div>
          <label for="email" class="input-label">{{ t('auth.emailLabel') }}</label>
          <div class="relative">
            <Icon name="mail" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              id="email"
              v-model="form.email"
              type="email"
              required
              autofocus
              autocomplete="email"
              :disabled="isLoading || codeSent"
              class="input pl-11"
              :placeholder="t('auth.emailPlaceholder')"
            />
          </div>
        </div>

        <template v-if="codeSent">
          <div>
            <label for="verifyCode" class="input-label">{{ t('auth.verificationCode') }}</label>
            <div class="relative">
              <Icon name="shield" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="verifyCode"
                v-model="form.verifyCode"
                type="text"
                required
                inputmode="numeric"
                autocomplete="one-time-code"
                maxlength="6"
                :disabled="isLoading"
                class="input pl-11"
                :placeholder="t('auth.verificationCodeHint')"
              />
            </div>
            <div class="mt-2 flex items-start justify-between gap-4 text-xs">
              <span class="text-gray-500 dark:text-dark-400">{{ t('auth.passwordResetCodeSentHint') }}</span>
              <button
                type="button"
                class="shrink-0 font-medium text-primary-600 hover:text-primary-500 disabled:cursor-not-allowed disabled:text-gray-400 dark:text-primary-400"
                :disabled="isLoading || countdown > 0"
                @click="sendCode"
              >
                {{ countdown > 0 ? t('auth.passwordResetResendCodeCountdown', { countdown }) : t('auth.passwordResetResendCode') }}
              </button>
            </div>
          </div>

          <div>
            <label for="password" class="input-label">{{ t('auth.newPassword') }}</label>
            <div class="relative">
              <Icon name="lock" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="password"
                v-model="form.password"
                :type="showPassword ? 'text' : 'password'"
                required
                autocomplete="new-password"
                :disabled="isLoading"
                class="input pl-11 pr-11"
                :placeholder="t('auth.newPasswordPlaceholder')"
              />
              <button type="button" class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-gray-400 hover:text-gray-600" :aria-label="t('auth.newPassword')" @click="showPassword = !showPassword">
                <Icon :name="showPassword ? 'eyeOff' : 'eye'" size="md" />
              </button>
            </div>
          </div>

          <div>
            <label for="confirmPassword" class="input-label">{{ t('auth.confirmPassword') }}</label>
            <div class="relative">
              <Icon name="lock" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                id="confirmPassword"
                v-model="form.confirmPassword"
                :type="showConfirmPassword ? 'text' : 'password'"
                required
                autocomplete="new-password"
                :disabled="isLoading"
                class="input pl-11 pr-11"
                :placeholder="t('auth.confirmPasswordPlaceholder')"
              />
              <button type="button" class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-gray-400 hover:text-gray-600" :aria-label="t('auth.confirmPassword')" @click="showConfirmPassword = !showConfirmPassword">
                <Icon :name="showConfirmPassword ? 'eyeOff' : 'eye'" size="md" />
              </button>
            </div>
          </div>
        </template>

        <div v-if="turnstileEnabled && turnstileSiteKey && !codeSent">
          <TurnstileWidget
            ref="turnstileRef"
            :site-key="turnstileSiteKey"
            @verify="turnstileToken = $event"
            @expire="turnstileToken = ''"
            @error="turnstileToken = ''"
          />
        </div>

        <button type="submit" :disabled="isLoading || (!codeSent && turnstileEnabled && !turnstileToken)" class="btn btn-primary w-full">
          <Icon :name="codeSent ? 'checkCircle' : 'mail'" size="md" class="mr-2" />
          {{ submitText }}
        </button>
      </form>
    </div>

    <template #footer>
      <p class="text-gray-500 dark:text-dark-400">
        {{ t('auth.rememberedPassword') }}
        <router-link to="/login" class="font-medium text-primary-600 dark:text-primary-400">{{ t('auth.signIn') }}</router-link>
      </p>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { useAppStore } from '@/stores'
import { forgotPassword, getPublicSettings, resetPassword } from '@/api/auth'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const isLoading = ref(false)
const codeSent = ref(false)
const countdown = ref(0)
const showPassword = ref(false)
const showConfirmPassword = ref(false)
const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const turnstileToken = ref('')
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const form = reactive({ email: '', verifyCode: '', password: '', confirmPassword: '' })
let timer: ReturnType<typeof setInterval> | undefined

const submitText = computed(() => {
  if (isLoading.value) return codeSent.value ? t('auth.resettingPassword') : t('auth.sendingVerificationCode')
  return codeSent.value ? t('auth.resetPassword') : t('auth.sendVerificationCode')
})

onMounted(async () => {
  try {
    const settings = await getPublicSettings()
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
})

onUnmounted(() => { if (timer) clearInterval(timer) })

function startCountdown(seconds: number): void {
  if (timer) clearInterval(timer)
  countdown.value = seconds
  timer = setInterval(() => {
    countdown.value = Math.max(0, countdown.value - 1)
    if (countdown.value === 0 && timer) clearInterval(timer)
  }, 1000)
}

async function sendCode(): Promise<void> {
  const email = form.email.trim()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
    appStore.showError(t('auth.invalidEmail'))
    return
  }
  isLoading.value = true
  try {
    const result = await forgotPassword({ email, turnstile_token: turnstileToken.value || undefined })
    codeSent.value = true
    form.verifyCode = ''
    startCountdown(result.countdown)
    appStore.showSuccess(t('auth.passwordResetCodeSent'))
  } catch (error: unknown) {
    turnstileRef.value?.reset()
    turnstileToken.value = ''
    const err = error as { message?: string }
    appStore.showError(err.message || t('auth.sendVerificationCodeFailed'))
  } finally {
    isLoading.value = false
  }
}

async function handleSubmit(): Promise<void> {
  if (!codeSent.value) {
    await sendCode()
    return
  }
  if (!/^\d{6}$/.test(form.verifyCode.trim())) {
    appStore.showError(t('auth.invalidVerificationCode'))
    return
  }
  if (form.password.length < 8) {
    appStore.showError(t('auth.passwordMinLength'))
    return
  }
  if (form.password !== form.confirmPassword) {
    appStore.showError(t('auth.passwordsDoNotMatch'))
    return
  }
  isLoading.value = true
  try {
    await resetPassword({ email: form.email.trim(), verify_code: form.verifyCode.trim(), new_password: form.password })
    appStore.showSuccess(t('auth.passwordResetSuccess'))
    await router.push('/login')
  } catch (error: unknown) {
    const err = error as { message?: string; code?: string }
    appStore.showError(err.code === 'INVALID_VERIFY_CODE' ? t('auth.invalidVerificationCode') : err.message || t('auth.resetPasswordFailed'))
  } finally {
    isLoading.value = false
  }
}
</script>
