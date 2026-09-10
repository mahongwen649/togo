import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ResetPasswordView from '../ResetPasswordView.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('ResetPasswordView', () => {
  it('directs legacy reset links to the verification-code flow', () => {
    const wrapper = mount(ResetPasswordView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /></div>' },
          Icon: true,
          RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' },
        },
      },
    })

    expect(wrapper.find('#password').exists()).toBe(false)
    expect(wrapper.get('a').attributes('href')).toBe('/forgot-password')
    expect(wrapper.text()).toContain('auth.passwordResetUsesCode')
  })
})
