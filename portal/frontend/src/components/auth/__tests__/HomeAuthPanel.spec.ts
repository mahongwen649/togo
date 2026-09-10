import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import HomeAuthPanel from '@/components/auth/HomeAuthPanel.vue'

vi.mock('@/views/auth/LoginView.vue', () => ({
  default: {
    name: 'LoginView',
    props: ['embedded'],
    template: '<div data-testid="embedded-login">embedded login</div>',
  },
}))

vi.mock('@/views/auth/RegisterView.vue', () => ({
  default: {
    name: 'RegisterView',
    props: ['embedded'],
    template: '<div data-testid="embedded-register">embedded register</div>',
  },
}))

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
  routerPush: vi.fn(),
}))

const authState = vi.hoisted(() => ({
  isAuthenticated: false,
  isAdmin: false,
  canAccessAdminArea: false,
  user: null as null | { email?: string; username?: string },
  login: vi.fn(),
  register: vi.fn(),
}))

const appState = vi.hoisted(() => ({
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
}))

const publicSettingsState = vi.hoisted(() => ({
  getPublicSettings: vi.fn(),
  sendVerifyCode: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    push: routeState.routerPush,
  }),
  RouterLink: {
    props: ['to'],
    template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
  },
}))


vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const messages: Record<string, string> = {
        'auth.homePanel.settingsLoadFailed': '注册配置加载失败，请刷新页面后重试。',
        'auth.homePanel.startTitle': '开始使用 TogoAPI',
        'auth.homePanel.loginTitle': '登录你的账号',
        'auth.homePanel.registerTab': '没账号，注册',
        'auth.homePanel.loginTab': '已有账号，登录',
        'auth.homePanel.registerSuccess': '注册成功，已自动登录',
        'auth.homePanel.suffixOnly': '仅允许 {suffixes} 邮箱注册',
      }
      let value = messages[key] || key
      for (const [name, replacement] of Object.entries(params || {})) {
        value = value.replace(`{${name}}`, String(replacement))
      }
      return value
    },
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => authState,
  useAppStore: () => appState,
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: publicSettingsState.getPublicSettings,
  sendVerifyCode: publicSettingsState.sendVerifyCode,
  isTotp2FARequired: (response: { requires_2fa?: boolean }) => response.requires_2fa === true,
}))

vi.mock('@/utils/authError', () => ({
  buildAuthErrorMessage: () => '注册失败',
}))

vi.mock('@/utils/apiError', () => ({
  extractI18nErrorMessage: () => '登录失败',
}))

describe('HomeAuthPanel', () => {
  beforeEach(() => {
    localStorage.clear()
    routeState.query = {}
    routeState.routerPush.mockReset()
    authState.isAuthenticated = false
    authState.isAdmin = false
    authState.canAccessAdminArea = false
    authState.user = null
    authState.login.mockReset()
    authState.register.mockReset()
    authState.register.mockResolvedValue({})
    appState.showError.mockReset()
    appState.showWarning.mockReset()
    appState.showSuccess.mockReset()
    publicSettingsState.sendVerifyCode.mockReset()
    publicSettingsState.sendVerifyCode.mockResolvedValue({ message: 'sent', countdown: 60 })
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      login_agreement_enabled: false,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: [],
    })
  })

  it('defaults to the register form for visitors without an account', async () => {
    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('开始使用 TogoAPI')
    expect(wrapper.find('#home-register-email').exists()).toBe(true)
    expect(wrapper.find('#home-login-identifier').exists()).toBe(false)
  })

  it('keeps registration unavailable when public settings fail to load', async () => {
    publicSettingsState.getPublicSettings.mockRejectedValue(new Error('network error'))

    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('注册配置加载失败，请刷新页面后重试。')
    expect(wrapper.find('#home-register-email').exists()).toBe(false)
    expect(wrapper.find('button[type="submit"]').exists()).toBe(false)
  })

  it('switches to the login form when the existing-account control is clicked', async () => {
    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })

    await wrapper.findAll('button').find((button) => button.text().includes('已有账号'))?.trigger('click')

    expect(wrapper.text()).toContain('登录你的账号')
    expect(wrapper.find('#home-login-identifier').exists()).toBe(true)
    expect(wrapper.find('#home-register-email').exists()).toBe(false)
  })

  it('shows the password reset entry when Portal reports it available', async () => {
    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('已有账号'))?.trigger('click')

    expect(wrapper.find('a[href="/forgot-password"]').exists()).toBe(true)
  })

  it('sends a verification code and registers from the same home panel', async () => {
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: true,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      login_agreement_enabled: false,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: ['@qq.com'],
    })

    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.find('#home-register-username').setValue('jqcode_user')
    await wrapper.find('#home-register-email').setValue('user@qq.com')
    await wrapper.find('#home-register-password').setValue('sub2api123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(publicSettingsState.sendVerifyCode).toHaveBeenCalledWith({
      email: 'user@qq.com',
      turnstile_token: undefined,
    })
    expect(wrapper.find('#home-register-verify-code').exists()).toBe(true)

    await wrapper.find('#home-register-verify-code').setValue('123456')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authState.register).toHaveBeenCalledWith({
      username: 'jqcode_user',
      email: 'user@qq.com',
      password: 'sub2api123',
      verify_code: '123456',
    })
    expect(appState.showSuccess).toHaveBeenLastCalledWith('注册成功，已自动登录')
    expect(routeState.routerPush).toHaveBeenCalledWith('/dashboard')
  })

  it('submits the affiliate code from an invite link', async () => {
    routeState.query = { aff: 'AFF123' }
    const wrapper = mount(HomeAuthPanel, {
      global: { stubs: { Icon: true, RouterLink: true } }
    })
    await flushPromises()

    const affiliateCodeInput = wrapper.find('#home-register-affiliate-code')
    expect(affiliateCodeInput.exists()).toBe(true)
    expect(affiliateCodeInput.attributes('readonly')).toBeUndefined()
    expect((affiliateCodeInput.element as HTMLInputElement).value).toBe('AFF123')

    await wrapper.find('#home-register-username').setValue('invited_user')
    await wrapper.find('#home-register-email').setValue('invited@example.com')
    await wrapper.find('#home-register-password').setValue('sub2api123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authState.register).toHaveBeenCalledWith({
      username: 'invited_user',
      email: 'invited@example.com',
      password: 'sub2api123',
      aff_code: 'AFF123'
    })
  })

  it('submits registration and affiliate codes separately', async () => {
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      invitation_code_enabled: true,
      turnstile_enabled: false,
      login_agreement_enabled: false,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: [],
    })

    const wrapper = mount(HomeAuthPanel, {
      global: { stubs: { Icon: true, RouterLink: true } }
    })
    await flushPromises()

    await wrapper.find('#home-register-username').setValue('registered_user')
    await wrapper.find('#home-register-email').setValue('registered@example.com')
    await wrapper.find('#home-register-password').setValue('sub2api123')
    await wrapper.find('#home-register-invitation-code').setValue('REG123')
    await wrapper.find('#home-register-affiliate-code').setValue('AFF123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authState.register).toHaveBeenCalledWith({
      username: 'registered_user',
      email: 'registered@example.com',
      password: 'sub2api123',
      invitation_code: 'REG123',
      aff_code: 'AFF123',
    })
  })

  it('does not restore an affiliate code from a previous visit', async () => {
    localStorage.setItem('affiliate_referral_code', JSON.stringify({
      code: 'STALE123',
      expiresAt: Date.now() + 60_000,
    }))

    const wrapper = mount(HomeAuthPanel, {
      global: { stubs: { Icon: true, RouterLink: true } }
    })
    await flushPromises()

    expect((wrapper.find('#home-register-affiliate-code').element as HTMLInputElement).value).toBe('')
  })

  it('submits a manually entered affiliate code', async () => {
    const wrapper = mount(HomeAuthPanel, {
      global: { stubs: { Icon: true, RouterLink: true } }
    })
    await flushPromises()

    await wrapper.find('#home-register-username').setValue('manual_invite')
    await wrapper.find('#home-register-email').setValue('manual@example.com')
    await wrapper.find('#home-register-password').setValue('sub2api123')
    await wrapper.find('#home-register-affiliate-code').setValue('  MANUAL123  ')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authState.register).toHaveBeenCalledWith({
      username: 'manual_invite',
      email: 'manual@example.com',
      password: 'sub2api123',
      aff_code: 'MANUAL123',
    })
  })

  it('rejects seven-character passwords during registration', async () => {
    const wrapper = mount(HomeAuthPanel, {
      global: { stubs: { Icon: true, RouterLink: true } }
    })
    await flushPromises()

    await wrapper.find('#home-register-username').setValue('short_password')
    await wrapper.find('#home-register-email').setValue('short@example.com')
    await wrapper.find('#home-register-password').setValue('1234567')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authState.register).not.toHaveBeenCalled()
    expect(appState.showError).toHaveBeenCalledWith('auth.homePanel.passwordMin')
  })

  it('redirects company managers to the management dashboard after inline login', async () => {
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      login_agreement_enabled: false,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: [],
    })
    authState.login.mockImplementation(async () => {
      authState.canAccessAdminArea = true
    })

    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text().includes('已有账号'))?.trigger('click')
    await wrapper.find('#home-login-identifier').setValue('user3')
    await wrapper.find('#home-login-password').setValue('12345678')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(routeState.routerPush).toHaveBeenCalledWith('/admin/dashboard')
  })

  it('blocks disallowed email suffixes before sending a verification code', async () => {
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: true,
      invitation_code_enabled: false,
      turnstile_enabled: false,
      login_agreement_enabled: false,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: ['@qq.com'],
    })

    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    await wrapper.find('#home-register-username').setValue('jqcode_user')
    await wrapper.find('#home-register-email').setValue('user@gmail.com')
    await wrapper.find('#home-register-password').setValue('sub2api123')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(publicSettingsState.sendVerifyCode).not.toHaveBeenCalled()
    expect(appState.showError).toHaveBeenCalledWith(expect.stringContaining('@qq.com'))
  })

  it('keeps complex registration and login inside the home panel', async () => {
    publicSettingsState.getPublicSettings.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: true,
      invitation_code_enabled: true,
      turnstile_enabled: true,
      login_agreement_enabled: true,
      password_reset_enabled: true,
      registration_email_suffix_whitelist: [],
    })

    const wrapper = mount(HomeAuthPanel, {
      global: {
        stubs: {
          Icon: true,
          LoginView: { template: '<div data-testid="embedded-login">embedded login</div>' },
          RegisterView: { template: '<div data-testid="embedded-register">embedded register</div>' },
          RouterLink: {
            props: ['to'],
            template: '<a :href="typeof to === `string` ? to : `#`"><slot /></a>',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="embedded-register"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/register"]').exists()).toBe(false)

    await wrapper.findAll('button').find((button) => button.text().includes('已有账号'))?.trigger('click')

    expect(wrapper.find('[data-testid="embedded-login"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/login"]').exists()).toBe(false)
    expect(authState.login).not.toHaveBeenCalled()
    expect(authState.register).not.toHaveBeenCalled()
  })
})
