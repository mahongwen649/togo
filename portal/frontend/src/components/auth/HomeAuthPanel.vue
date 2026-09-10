<template>
  <div class="home-auth-card">
    <div class="home-auth-heading relative z-10 mb-6">
      <div>
        <p class="text-xs font-semibold uppercase tracking-wide text-teal-600 dark:text-teal-300">
          {{ isAuthenticated ? 'Account Ready' : activeMode === 'register' ? 'Create Access' : 'Welcome Back' }}
        </p>
        <h2 class="mt-1 text-2xl font-bold text-slate-950 dark:text-white">
          {{ panelTitle }}
        </h2>
      </div>
    </div>

    <div v-if="isAuthenticated" class="relative z-10 space-y-5">
      <div class="rounded-3xl border border-white/80 bg-white/60 p-4 shadow-sm shadow-teal-500/10 backdrop-blur dark:border-white/10 dark:bg-white/5">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-r from-teal-500 to-teal-600 text-sm font-semibold text-white">
            {{ userInitial }}
          </div>
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold text-slate-900 dark:text-white">
              {{ userEmail }}
            </p>
            <p class="text-xs text-slate-500 dark:text-slate-400">
              {{ t('auth.homePanel.loggedIn') }}
            </p>
          </div>
        </div>
      </div>
      <router-link :to="dashboardPath" class="home-auth-primary w-full justify-center py-3">
        {{ t('auth.homePanel.dashboard') }}
        <Icon name="arrowRight" size="md" class="ml-2" />
      </router-link>
    </div>

    <div v-else class="relative z-10">
      <div class="home-auth-tabs mb-5 grid grid-cols-2 rounded-full border border-white/80 bg-slate-100/75 p-1 shadow-inner shadow-teal-500/5 backdrop-blur dark:border-white/10 dark:bg-dark-900/70">
        <button
          type="button"
          class="rounded-full px-3 py-2 text-sm font-semibold transition"
          :class="activeMode === 'register'
            ? 'bg-white text-teal-700 shadow-sm shadow-teal-500/10 dark:bg-dark-950 dark:text-teal-200'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white'"
          @click="activeMode = 'register'"
        >
          {{ t('auth.homePanel.registerTab') }}
        </button>
        <button
          type="button"
          class="rounded-full px-3 py-2 text-sm font-semibold transition"
          :class="activeMode === 'login'
            ? 'bg-white text-teal-700 shadow-sm shadow-teal-500/10 dark:bg-dark-950 dark:text-teal-200'
            : 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-white'"
          @click="activeMode = 'login'"
        >
          {{ t('auth.homePanel.loginTab') }}
        </button>
      </div>

      <form v-if="activeMode === 'register' && !registerNeedsFullPage" class="home-auth-form space-y-4" @submit.prevent="handleRegister">
        <div v-if="settingsLoadFailed" class="rounded-2xl border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700 dark:border-rose-800/60 dark:bg-rose-900/20 dark:text-rose-300">
          {{ t('auth.homePanel.settingsLoadFailed') }}
        </div>

        <div v-else-if="!registrationEnabled && settingsLoaded" class="rounded-2xl border border-amber-200 bg-amber-50 p-3 text-sm text-amber-700 dark:border-amber-800/60 dark:bg-amber-900/20 dark:text-amber-300">
          {{ t('auth.homePanel.registrationClosed') }}
        </div>

        <template v-else>
          <div>
            <label for="home-register-username" class="input-label">{{ t('auth.homePanel.username') }}</label>
            <div class="relative">
              <Icon name="user" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                id="home-register-username"
                v-model="registerForm.username"
                type="text"
                class="input pl-11"
                autocomplete="username"
                :placeholder="t('auth.homePanel.usernamePlaceholder')"
                :disabled="actionDisabled"
              />
            </div>
          </div>

          <div>
            <label for="home-register-email" class="input-label">{{ t('auth.homePanel.email') }}</label>
            <div class="relative">
              <Icon name="mail" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                id="home-register-email"
                v-model="registerForm.email"
                type="email"
                class="input pl-11"
                autocomplete="email"
                placeholder="name@example.com"
                :disabled="actionDisabled"
              />
            </div>
            <p v-if="emailWhitelistHint" class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
              {{ emailWhitelistHint }}
            </p>
          </div>

          <div>
            <label for="home-register-password" class="input-label">{{ t('auth.homePanel.password') }}</label>
            <div class="relative">
              <Icon name="lock" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                id="home-register-password"
                v-model="registerForm.password"
                :type="showRegisterPassword ? 'text' : 'password'"
                class="input pl-11 pr-11"
                autocomplete="new-password"
                :placeholder="t('auth.homePanel.passwordPlaceholder')"
                :disabled="actionDisabled"
              />
              <button
                type="button"
                class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-slate-400 hover:text-slate-600"
                :disabled="actionDisabled"
                @click="showRegisterPassword = !showRegisterPassword"
              >
                <Icon :name="showRegisterPassword ? 'eyeOff' : 'eye'" size="md" />
              </button>
            </div>
          </div>

          <div v-if="invitationCodeEnabled">
            <label for="home-register-invitation-code" class="input-label">{{ t('auth.registrationCodeLabel') }}</label>
            <div class="relative">
              <Icon name="key" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                id="home-register-invitation-code"
                v-model="invitationCode"
                type="text"
                class="input pl-11"
                autocomplete="off"
                :placeholder="t('auth.registrationCodePlaceholder')"
                :disabled="actionDisabled"
              />
            </div>
          </div>

          <div>
            <label for="home-register-affiliate-code" class="input-label">{{ t('auth.affiliateCodeLabel') }}</label>
            <div class="relative">
              <Icon name="key" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                id="home-register-affiliate-code"
                v-model="affiliateCode"
                type="text"
                class="input pl-11"
                autocomplete="off"
                :placeholder="t('auth.affiliateCodePlaceholder')"
                :disabled="actionDisabled"
              />
            </div>
          </div>

          <transition name="home-auth-expand">
            <div v-if="emailVerifyEnabled && codeSent" class="space-y-3 rounded-2xl border border-teal-100 bg-teal-50/70 p-3 dark:border-teal-900/60 dark:bg-teal-950/30">
              <div>
                <label for="home-register-verify-code" class="input-label">{{ t('auth.homePanel.verifyCode') }}</label>
                <input
                  id="home-register-verify-code"
                  v-model="verificationCode"
                  type="text"
                  class="input py-3 text-center font-mono text-lg tracking-[0.4em]"
                  autocomplete="one-time-code"
                  inputmode="numeric"
                  maxlength="6"
                  placeholder="000000"
                  :disabled="actionDisabled"
                />
                <p class="mt-2 text-center text-xs leading-5 text-teal-700 dark:text-teal-200">
                  {{ t('auth.homePanel.codeSent', { email: normalizedRegisterEmail }) }}
                </p>
              </div>

              <div class="flex items-center justify-center">
                <button
                  v-if="countdown > 0"
                  type="button"
                  disabled
                  class="text-xs font-medium text-slate-400 dark:text-slate-500"
                >
                  {{ t('auth.homePanel.resendAfter', { countdown }) }}
                </button>
                <button
                  v-else
                  type="button"
                  class="text-xs font-semibold text-teal-700 hover:text-teal-600 disabled:cursor-not-allowed disabled:opacity-50 dark:text-teal-200"
                  :disabled="isSendingCode"
                  @click="handleResendCode"
                >
                  {{ isSendingCode ? t('auth.homePanel.sending') : t('auth.homePanel.resend') }}
                </button>
              </div>
            </div>
          </transition>

          <button
            type="submit"
            class="home-auth-primary w-full justify-center py-3"
            :disabled="registerActionDisabled"
          >
            <span v-if="isLoading || isSendingCode" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
            <Icon v-else name="userPlus" size="md" class="mr-2" />
            {{ registerButtonText }}
          </button>
        </template>
      </form>

      <RegisterView v-else-if="activeMode === 'register'" embedded />

      <form v-else-if="!loginNeedsFullPage" class="home-auth-form space-y-4" @submit.prevent="handleLogin">
        <div>
          <label for="home-login-identifier" class="input-label">{{ t('auth.homePanel.account') }}</label>
          <div class="relative">
            <Icon name="user" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              id="home-login-identifier"
              v-model="loginForm.identifier"
              type="text"
              class="input pl-11"
              autocomplete="username"
              :placeholder="t('auth.homePanel.accountPlaceholder')"
              :disabled="actionDisabled"
            />
          </div>
        </div>

        <div>
          <div class="mb-1.5 flex items-center justify-between">
            <label for="home-login-password" class="input-label mb-0">{{ t('auth.homePanel.password') }}</label>
            <router-link
              v-if="passwordResetEnabled"
              to="/forgot-password"
              class="text-xs font-medium text-teal-600 hover:text-teal-500 dark:text-teal-300"
            >
              {{ t('auth.homePanel.forgotPassword') }}
            </router-link>
          </div>
          <div class="relative">
            <Icon name="lock" size="md" class="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              id="home-login-password"
              v-model="loginForm.password"
              :type="showLoginPassword ? 'text' : 'password'"
              class="input pl-11 pr-11"
              autocomplete="current-password"
              :placeholder="t('auth.homePanel.inputPassword')"
              :disabled="actionDisabled"
            />
            <button
              type="button"
              class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-slate-400 hover:text-slate-600"
              :disabled="actionDisabled"
              @click="showLoginPassword = !showLoginPassword"
            >
              <Icon :name="showLoginPassword ? 'eyeOff' : 'eye'" size="md" />
            </button>
          </div>
        </div>

        <button type="submit" class="home-auth-primary w-full justify-center py-3" :disabled="loginActionDisabled">
          <span v-if="isLoading" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
          <Icon v-else name="login" size="md" class="mr-2" />
          {{ t('auth.homePanel.login') }}
        </button>
      </form>

      <LoginView v-else embedded />

      <p class="mt-4 text-center text-xs leading-5 text-slate-500 dark:text-slate-400">
        {{ activeMode === 'register' ? t('auth.homePanel.registerHint') : t('auth.homePanel.loginHint') }}
      </p>
    </div>

  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import LoginView from '@/views/auth/LoginView.vue'
import RegisterView from '@/views/auth/RegisterView.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { getPublicSettings, sendVerifyCode } from '@/api/auth'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { validateNewUsername } from '@/utils/usernamePolicy'
import {
  formatRegistrationEmailSuffixWhitelistForMessage,
  isRegistrationEmailSuffixAllowed,
  normalizeRegistrationEmailSuffixWhitelist
} from '@/utils/registrationEmailPolicy'
import { pickOAuthAffiliateCode } from '@/utils/oauthAffiliate'
import { meetsPasswordLength } from '@/utils/passwordPolicy'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const appStore = useAppStore()

const activeMode = ref<'register' | 'login'>(route.query.mode === 'login' ? 'login' : 'register')
const isLoading = ref(false)
const isSendingCode = ref(false)
const settingsLoaded = ref(false)
const settingsLoadFailed = ref(false)
const showLoginPassword = ref(false)
const showRegisterPassword = ref(false)
const registrationEnabled = ref(true)
const invitationCodeEnabled = ref(false)
const emailVerifyEnabled = ref(false)
const turnstileEnabled = ref(false)
const loginAgreementEnabled = ref(false)
const passwordResetEnabled = ref(false)
const registrationEmailSuffixWhitelist = ref<string[]>([])
const verificationCode = ref('')
const invitationCode = ref('')
const affiliateCode = ref('')
const codeSent = ref(false)
const countdown = ref(0)
let countdownTimer: ReturnType<typeof setInterval> | null = null

const loginForm = reactive({
  identifier: '',
  password: ''
})

const registerForm = reactive({
  username: '',
  email: '',
  password: ''
})

const isAuthenticated = computed(() => authStore.isAuthenticated)
const dashboardPath = computed(() => authStore.canAccessAdminArea ? '/admin/dashboard' : '/dashboard')
const userEmail = computed(() => authStore.user?.email || authStore.user?.username || '')
const userInitial = computed(() => (userEmail.value || 'U').charAt(0).toUpperCase())
const normalizedRegisterEmail = computed(() => registerForm.email.trim())
const panelTitle = computed(() => {
  if (isAuthenticated.value) return t('auth.homePanel.readyTitle')
  return activeMode.value === 'register' ? t('auth.homePanel.startTitle') : t('auth.homePanel.loginTitle')
})
const actionDisabled = computed(() => isLoading.value || !settingsLoaded.value || settingsLoadFailed.value)
const registerNeedsFullPage = computed(() => turnstileEnabled.value || loginAgreementEnabled.value)
const loginNeedsFullPage = computed(() => turnstileEnabled.value || loginAgreementEnabled.value)
const registerActionDisabled = computed(() => actionDisabled.value || isSendingCode.value || registerNeedsFullPage.value)
const loginActionDisabled = computed(() => actionDisabled.value || loginNeedsFullPage.value)
const registerButtonText = computed(() => {
  if (isSendingCode.value) return t('auth.homePanel.sending')
  if (isLoading.value) return t('auth.homePanel.processing')
  if (emailVerifyEnabled.value) return codeSent.value ? t('auth.homePanel.verifyAndCreate') : t('auth.homePanel.sendCode')
  return t('auth.homePanel.createAccount')
})
const emailWhitelistHint = computed(() => {
  const normalizedWhitelist = normalizeRegistrationEmailSuffixWhitelist(registrationEmailSuffixWhitelist.value)
  if (normalizedWhitelist.length === 0) return ''
  const suffixes = formatRegistrationEmailSuffixWhitelistForMessage(normalizedWhitelist, {
	separator: '、',
	more: (count) => t('auth.homePanel.suffixMore', { count })
	})
  return t('auth.homePanel.suffixOnly', { suffixes })
})

onMounted(async () => {
  affiliateCode.value = pickOAuthAffiliateCode(route.query.aff, route.query.aff_code)
  try {
    const settings = await getPublicSettings()
    registrationEnabled.value = settings.registration_enabled
    invitationCodeEnabled.value = settings.invitation_code_enabled === true
    emailVerifyEnabled.value = settings.email_verify_enabled
    turnstileEnabled.value = settings.turnstile_enabled
    loginAgreementEnabled.value = settings.login_agreement_enabled === true
    passwordResetEnabled.value = settings.password_reset_enabled
    registrationEmailSuffixWhitelist.value = normalizeRegistrationEmailSuffixWhitelist(
      settings.registration_email_suffix_whitelist || []
    )
  } catch (error) {
    console.error('Failed to load home auth settings:', error)
    settingsLoadFailed.value = true
  } finally {
    settingsLoaded.value = !settingsLoadFailed.value
  }
})

watch(
  () => route.query.mode,
  (mode) => {
    activeMode.value = mode === 'login' ? 'login' : 'register'
  }
)

watch(
  () => registerForm.email,
  () => {
    if (codeSent.value) {
      codeSent.value = false
      verificationCode.value = ''
      stopCountdown()
    }
  }
)

onUnmounted(() => {
  stopCountdown()
})

function validateLogin(): boolean {
  if (!loginForm.identifier.trim()) {
    appStore.showError(t('auth.homePanel.enterAccount'))
    return false
  }
  if (!meetsPasswordLength(loginForm.password)) {
    appStore.showError(t('auth.homePanel.passwordMin'))
    return false
  }
  return true
}

function validateRegister(): boolean {
  if (!registerForm.username.trim()) {
	appStore.showError(t('auth.homePanel.enterUsername'))
	return false
  }
  if (!validateNewUsername(registerForm.username)) {
    appStore.showError(t('auth.homePanel.invalidUsername'))
    return false
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(registerForm.email)) {
    appStore.showError(t('auth.homePanel.invalidEmail'))
    return false
  }
  if (!isRegistrationEmailSuffixAllowed(registerForm.email, registrationEmailSuffixWhitelist.value)) {
    appStore.showError(buildEmailSuffixNotAllowedMessage())
    return false
  }
  if (!meetsPasswordLength(registerForm.password)) {
    appStore.showError(t('auth.homePanel.passwordMin'))
    return false
  }
  if (invitationCodeEnabled.value && !invitationCode.value.trim()) {
    appStore.showError(t('auth.invitationCodeRequired'))
    return false
  }
  return true
}

function validateVerificationCode(): boolean {
  if (!/^\d{6}$/.test(verificationCode.value.trim())) {
    appStore.showError(t('auth.homePanel.invalidCode'))
    return false
  }
  return true
}

function buildEmailSuffixNotAllowedMessage(): string {
  const normalizedWhitelist = normalizeRegistrationEmailSuffixWhitelist(registrationEmailSuffixWhitelist.value)
  if (normalizedWhitelist.length === 0) {
    return t('auth.homePanel.suffixBlocked')
  }
  const suffixes = formatRegistrationEmailSuffixWhitelistForMessage(normalizedWhitelist, {
	separator: '、',
	more: (count) => t('auth.homePanel.suffixMore', { count })
	})
  return t('auth.homePanel.suffixOnly', { suffixes })
}

function startCountdown(seconds: number): void {
  countdown.value = seconds
  stopCountdown()
  countdownTimer = setInterval(() => {
    if (countdown.value > 0) {
      countdown.value -= 1
      return
    }
    stopCountdown()
  }, 1000)
}

function stopCountdown(): void {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  if (countdown.value < 0) {
    countdown.value = 0
  }
}

async function handleLogin(): Promise<void> {
  if (!validateLogin()) return

  isLoading.value = true
  try {
    await authStore.login({
      identifier: loginForm.identifier.trim(),
      password: loginForm.password
    })
    appStore.showSuccess(t('auth.homePanel.loginSuccess'))
    await router.push((route.query.redirect as string) || dashboardPath.value)
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'auth.errors', t('auth.homePanel.loginFailed')))
  } finally {
    isLoading.value = false
  }
}

async function handleRegister(): Promise<void> {
  if (!settingsLoaded.value || settingsLoadFailed.value) {
    appStore.showError(t('auth.homePanel.settingsLoadFailed'))
    return
  }

  if (!validateRegister()) return

  if (emailVerifyEnabled.value && !codeSent.value) {
    await sendRegistrationCode()
    return
  }

  if (emailVerifyEnabled.value && !validateVerificationCode()) return

  isLoading.value = true
  try {
    const normalizedInvitationCode = invitationCode.value.trim()
    const normalizedAffiliateCode = affiliateCode.value.trim()
    await authStore.register({
      username: registerForm.username.trim(),
      email: registerForm.email.trim(),
      password: registerForm.password,
      ...(normalizedInvitationCode ? { invitation_code: normalizedInvitationCode } : {}),
      ...(normalizedAffiliateCode ? { aff_code: normalizedAffiliateCode } : {}),
      ...(emailVerifyEnabled.value ? { verify_code: verificationCode.value.trim() } : {})
    })
    appStore.showSuccess(t('auth.homePanel.registerSuccess'))
    await router.push(dashboardPath.value)
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'auth.errors', t('auth.homePanel.registerFailed')))
  } finally {
    isLoading.value = false
  }
}

async function sendRegistrationCode(): Promise<void> {
  isSendingCode.value = true
  try {
    const response = await sendVerifyCode({
      email: normalizedRegisterEmail.value,
      turnstile_token: undefined
    })
    codeSent.value = true
    verificationCode.value = ''
    startCountdown(response.countdown)
    appStore.showSuccess(t('auth.homePanel.codeSendSuccess'))
  } catch (error) {
    appStore.showError(extractI18nErrorMessage(error, t, 'auth.errors', t('auth.homePanel.codeSendFailed')))
  } finally {
    isSendingCode.value = false
  }
}

async function handleResendCode(): Promise<void> {
  if (countdown.value > 0 || isSendingCode.value) return
  if (!validateRegister()) return
  await sendRegistrationCode()
}
</script>

<style scoped>
.home-auth-card {
  position: relative;
  overflow: hidden;
  width: 100%;
  max-width: 480px;
  border-radius: 1.8rem;
  border: 1px solid rgba(255, 255, 255, 0.78);
  background: rgba(255, 255, 255, 0.74);
  padding: clamp(1.35rem, 3vw, 1.85rem);
  box-shadow: 0 24px 64px -26px rgba(13, 148, 136, 0.48);
  backdrop-filter: blur(22px);
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease;
}

.home-auth-card::before {
  content: '';
  position: absolute;
  inset: -30%;
  pointer-events: none;
  background: radial-gradient(circle at 70% 24%, rgba(45, 212, 191, 0.2), transparent 28%);
  animation: auth-ambient 12s ease-in-out infinite alternate;
}

.home-auth-card::after {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(145deg, rgba(255, 255, 255, 0.42), transparent 48%);
}

.home-auth-card:hover {
  transform: translateY(-5px);
  box-shadow: 0 30px 72px -28px rgba(13, 148, 136, 0.56);
}

.home-auth-primary {
  display: inline-flex;
  align-items: center;
  border-radius: 9999px;
  background: linear-gradient(135deg, #14b8a6, #0d9488);
  color: #fff;
  font-weight: 800;
  box-shadow: 0 14px 34px rgba(20, 184, 166, 0.28);
  transition:
    transform 0.25s ease,
    box-shadow 0.25s ease,
    opacity 0.2s ease;
}

.home-auth-primary:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 22px 42px rgba(13, 148, 136, 0.32);
}

.home-auth-primary:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

:global(html.h5-density) .home-auth-card {
  max-width: 100%;
  padding: 0.85rem;
  border-radius: 1rem;
}

:global(html.h5-density) .home-auth-card:hover {
  transform: none;
}

:global(html.h5-density) .home-auth-heading {
  margin-bottom: 0.6rem;
}

:global(html.h5-density) .home-auth-heading h2 {
  margin-top: 0;
  font-size: 1.35rem;
  line-height: 1.65rem;
}

:global(html.h5-density) .home-auth-tabs {
  margin-bottom: 0.65rem;
}

:global(html.h5-density) .home-auth-tabs button {
  padding-top: 0.4rem;
  padding-bottom: 0.4rem;
  font-size: 0.78rem;
}

:global(html.h5-density) .home-auth-form {
  gap: 0.65rem;
}

:global(html.h5-density) .home-auth-primary {
  padding-top: 0.55rem;
  padding-bottom: 0.55rem;
  border-radius: 0.65rem;
}

@keyframes auth-ambient {
  0% {
    transform: translate3d(-2%, -1%, 0) scale(1);
    opacity: 0.55;
  }
  100% {
    transform: translate3d(2%, 2%, 0) scale(1.08);
    opacity: 0.82;
  }
}

:deep(.dark) .home-auth-card {
  border-color: rgba(255, 255, 255, 0.1);
  background: rgba(15, 23, 42, 0.66);
  box-shadow: 0 24px 60px -24px rgba(15, 118, 110, 0.5);
}

.home-auth-expand-enter-active,
.home-auth-expand-leave-active {
  transition:
    opacity 0.2s ease,
    transform 0.2s ease;
}

.home-auth-expand-enter-from,
.home-auth-expand-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
