import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ForgotPasswordView from '../ForgotPasswordView.vue'

const { forgotPasswordMock, resetPasswordMock, pushMock, showError } = vi.hoisted(() => ({
  forgotPasswordMock: vi.fn(),
  resetPasswordMock: vi.fn(),
  pushMock: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: pushMock }) }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/api/auth', () => ({
  getPublicSettings: vi.fn().mockResolvedValue({ turnstile_enabled: false }),
  forgotPassword: forgotPasswordMock,
  resetPassword: resetPasswordMock,
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('ForgotPasswordView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses the centered authentication layout without the brand panel', () => {
    const wrapper = mount(ForgotPasswordView, {
      global: {
        stubs: {
          AuthLayout: {
            props: ['showBrandPanel'],
            template: '<div data-testid="auth-layout" :data-show-brand-panel="showBrandPanel"><slot /></div>',
          },
          Icon: true,
          TurnstileWidget: true,
          RouterLink: true,
        },
      },
    })

    expect(wrapper.get('[data-testid="auth-layout"]').attributes('data-show-brand-panel')).toBe('false')
  })

  it('sends a code, then resets the password in the same form', async () => {
    forgotPasswordMock.mockResolvedValue({ countdown: 60, message: 'sent' })
    resetPasswordMock.mockResolvedValue({ message: 'reset' })
    const wrapper = mount(ForgotPasswordView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /></div>' },
          Icon: true,
          TurnstileWidget: true,
          RouterLink: true,
        },
      },
    })
    await flushPromises()
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(forgotPasswordMock).toHaveBeenCalledWith({ email: 'user@example.com', turnstile_token: undefined })

    await wrapper.get('#verifyCode').setValue('123456')
    await wrapper.get('#password').setValue('new-secret')
    await wrapper.get('#confirmPassword').setValue('new-secret')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(resetPasswordMock).toHaveBeenCalledWith({ email: 'user@example.com', verify_code: '123456', new_password: 'new-secret' })
    expect(pushMock).toHaveBeenCalledWith('/login')
  })

  it('rejects a seven-character replacement password', async () => {
    forgotPasswordMock.mockResolvedValue({ countdown: 60, message: 'sent' })
    const wrapper = mount(ForgotPasswordView, {
      global: { stubs: { AuthLayout: { template: '<div><slot /></div>' }, Icon: true, TurnstileWidget: true, RouterLink: true } },
    })
    await flushPromises()
    await wrapper.get('#email').setValue('user@example.com')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    await wrapper.get('#verifyCode').setValue('123456')
    await wrapper.get('#password').setValue('1234567')
    await wrapper.get('#confirmPassword').setValue('1234567')
    await wrapper.get('form').trigger('submit')
    expect(resetPasswordMock).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('auth.passwordMinLength')
  })
})
