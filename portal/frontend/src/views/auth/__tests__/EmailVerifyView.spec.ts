import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import EmailVerifyView from '@/views/auth/EmailVerifyView.vue'

const {
  pushMock,
  showSuccessMock,
  showErrorMock,
  registerMock,
  getPublicSettingsMock,
  sendVerifyCodeMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  registerMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  sendVerifyCodeMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
  }),
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (key === 'auth.accountCreatedSuccess') {
        return `Account created for ${params?.siteName ?? 'Sub2API'}`
      }
      if (key === 'auth.emailSuffixNotAllowedWithAllowed') {
        return `Allowed: ${params?.suffixes ?? ''}`
      }
      return key
    },
    locale: { value: 'en' },
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    register: (...args: any[]) => registerMock(...args),
  }),
  useAppStore: () => ({
    showSuccess: (...args: any[]) => showSuccessMock(...args),
    showError: (...args: any[]) => showErrorMock(...args),
  }),
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: any[]) => getPublicSettingsMock(...args),
    sendVerifyCode: (...args: any[]) => sendVerifyCodeMock(...args),
  }
})

function mountView() {
  return mount(EmailVerifyView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        TurnstileWidget: true,
        transition: false,
      },
    },
  })
}

describe('EmailVerifyView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showSuccessMock.mockReset()
    showErrorMock.mockReset()
    registerMock.mockReset()
    getPublicSettingsMock.mockReset()
    sendVerifyCodeMock.mockReset()
    sessionStorage.clear()
    localStorage.clear()

    getPublicSettingsMock.mockResolvedValue({
      turnstile_enabled: false,
      turnstile_site_key: '',
      site_name: 'Sub2API',
      registration_email_suffix_whitelist: [],
    })
    sendVerifyCodeMock.mockResolvedValue({ countdown: 60 })
    registerMock.mockResolvedValue({})
  })

  it('keeps the normal email registration verification flow', async () => {
    sessionStorage.setItem(
      'register_data',
      JSON.stringify({
        email: 'normal@example.com',
        username: 'normal-user',
        password: 'secret-456',
        promo_code: 'PROMO',
        invitation_code: 'INVITE',
      })
    )

    const wrapper = mountView()

    await flushPromises()
    await wrapper.get('#code').setValue('654321')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(sendVerifyCodeMock).toHaveBeenCalledWith({
      email: 'normal@example.com',
      turnstile_token: undefined,
    })
    expect(registerMock).toHaveBeenCalledWith({
      email: 'normal@example.com',
      username: 'normal-user',
      password: 'secret-456',
      verify_code: '654321',
      turnstile_token: undefined,
      promo_code: 'PROMO',
      invitation_code: 'INVITE',
    })
    expect(showSuccessMock).toHaveBeenCalledWith('Account created for Sub2API')
    expect(pushMock).toHaveBeenCalledWith('/dashboard')
  })

  it('blocks disallowed registration email suffixes before sending a code', async () => {
    getPublicSettingsMock.mockResolvedValue({
      turnstile_enabled: false,
      turnstile_site_key: '',
      site_name: 'Sub2API',
      registration_email_suffix_whitelist: ['allowed.com'],
    })
    sessionStorage.setItem(
      'register_data',
      JSON.stringify({
        email: 'normal@example.com',
        username: 'normal-user',
        password: 'secret-456',
      })
    )

    mountView()

    await flushPromises()

    expect(sendVerifyCodeMock).not.toHaveBeenCalled()
    expect(showErrorMock).toHaveBeenCalledWith('Allowed: @allowed.com')
  })
})
