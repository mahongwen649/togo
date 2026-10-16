import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountPlatformTypeCell from '../AccountPlatformTypeCell.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('AccountPlatformTypeCell', () => {
  it('uses the shared Core-style badge metadata and compact row', () => {
    const wrapper = mount(AccountPlatformTypeCell, {
      props: {
        account: {
          id: 682,
          name: 'OpenAI Plus',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          credentials: {
            auth_mode: 'agentIdentity',
            plan_type: 'plus',
            subscription_expires_at: '2026-10-13T00:00:00Z',
          },
          extra: {
            privacy_mode: 'training_set_failed',
            openai_compact_mode: 'auto',
          },
        },
      },
    })

    const badge = wrapper.getComponent(PlatformTypeBadge)
    expect(badge.props()).toMatchObject({
      platform: 'openai',
      type: 'oauth',
      authMode: 'agentIdentity',
      planType: 'plus',
      privacyMode: 'training_set_failed',
      subscriptionExpiresAt: '2026-10-13T00:00:00Z',
    })
    expect(wrapper.text()).toContain('admin.accounts.openai.compactAuto')
  })
})
