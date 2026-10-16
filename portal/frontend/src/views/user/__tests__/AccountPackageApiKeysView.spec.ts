import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import AccountPackageApiKeysView from '../AccountPackageApiKeysView.vue'
import { CC_SWITCH_USAGE_SCRIPT, OPENAI_CC_SWITCH_CODEX_MODEL } from '@/utils/ccswitchImport'

const api = vi.hoisted(() => ({
  list: vi.fn(), create: vi.fn(), delete: vi.fn(), toggleStatus: vi.fn(),
  packages: vi.fn(), groups: vi.fn(), settings: vi.fn()
}))
vi.mock('@/api/keys', () => ({ keysAPI: api }))
vi.mock('@/api/accountBusiness', () => ({ listAccountPackages: api.packages }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: api.groups } }))
vi.mock('@/api/auth', () => ({ authAPI: { getPublicSettings: api.settings } }))
vi.mock('@/api/url', () => ({ getPublicGatewayBaseURL: () => 'https://gateway.example.test' }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const key = { id: 23, name: 'Package key', key: 'sk-package-test-secret', group_id: 42, status: 'active', group: { id: 42, platform: 'openai' } }
const pkg = { id: 2, name: 'Plus', status: 'active', core_group_id: 42, accounts: [{ id: 679 }], expires_at: null }
let wrapper: VueWrapper

async function render() {
  wrapper = mount(AccountPackageApiKeysView, { global: { stubs: {
    BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' }
  } } })
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  api.list.mockResolvedValue({ items: [key, { ...key, id: 24, name: 'Regular API key', group_id: 99 }] })
  api.packages.mockResolvedValue([pkg])
  api.groups.mockResolvedValue([{ id: 42, platform: 'openai' }])
  api.settings.mockResolvedValue({ api_base_url: 'https://api.example.test', site_name: 'TogoAPI', hide_ccs_import_button: false })
  api.delete.mockResolvedValue(undefined)
  api.create.mockResolvedValue(key)
  vi.spyOn(window, 'open').mockReturnValue(null)
  vi.spyOn(document, 'hasFocus').mockReturnValue(false)
})

afterEach(() => {
  wrapper?.unmount()
  vi.restoreAllMocks()
})

describe('Account package key actions', () => {
  it('shows only CCS import and delete for each package key, keeping unrelated keys out', async () => {
    await render()
    const rows = wrapper.findAll('[data-test="package-key-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].findAll('button').map(button => button.text())).toEqual(['导入到 CCS', '删除'])
    expect(wrapper.text()).not.toContain('Regular API key')
    expect(wrapper.text()).not.toContain(key.key)
    expect(api.toggleStatus).not.toHaveBeenCalled()
  })

  it('imports the full key with the same gateway, Codex model and usage script as regular keys', async () => {
    await render()
    await wrapper.get('[aria-label="导入到 CCS"]').trigger('click')
    expect(window.open).toHaveBeenCalledTimes(1)
    const [link, target] = vi.mocked(window.open).mock.calls[0]
    const url = new URL(String(link))
    expect(url.protocol).toBe('ccswitch:')
    expect(target).toBe('_self')
    expect(url.searchParams.get('app')).toBe('codex')
    expect(url.searchParams.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(url.searchParams.get('endpoint')).toBe('https://api.example.test')
    expect(url.searchParams.get('name')).toBe('TogoAPI')
    expect(url.searchParams.get('apiKey')).toBe(key.key)
    expect(atob(url.searchParams.get('usageScript')!)).toBe(CC_SWITCH_USAGE_SCRIPT)
    expect(api.delete).not.toHaveBeenCalled()
    expect(api.create).not.toHaveBeenCalled()
  })

  it('still imports to Codex if a package key response omits its nested group', async () => {
    api.list.mockResolvedValue({ items: [{ ...key, group: undefined }] })
    api.groups.mockResolvedValue([])
    await render()
    await wrapper.get('[aria-label="导入到 CCS"]').trigger('click')
    const url = new URL(String(vi.mocked(window.open).mock.calls[0][0]))
    expect(url.searchParams.get('app')).toBe('codex')
  })

  it('uses the public gateway fallback when public settings cannot be fetched', async () => {
    api.settings.mockRejectedValue(new Error('offline'))
    await render()
    await wrapper.get('[aria-label="导入到 CCS"]').trigger('click')
    const url = new URL(String(vi.mocked(window.open).mock.calls[0][0]))
    expect(url.searchParams.get('endpoint')).toBe('https://gateway.example.test')
    expect(url.searchParams.get('name')).toBe('sub2api')
  })

  it('waits for settings before enabling import', async () => {
    let resolveSettings!: (settings: unknown) => void
    api.settings.mockReturnValue(new Promise(resolve => { resolveSettings = resolve }))
    await render()
    expect(wrapper.get('[aria-label="导入到 CCS"]').attributes('disabled')).toBeDefined()
    resolveSettings({ site_name: 'TogoAPI' })
    await flushPromises()
    expect(wrapper.get('[aria-label="导入到 CCS"]').attributes('disabled')).toBeUndefined()
  })

  it('respects the same CCS visibility setting as the original keys page', async () => {
    api.settings.mockResolvedValue({ hide_ccs_import_button: true })
    await render()
    expect(wrapper.find('[aria-label="导入到 CCS"]').exists()).toBe(false)
    expect(wrapper.get('[aria-label="删除 Key"]').exists()).toBe(true)
  })

  it('reports a protocol launch failure without exposing the key', async () => {
    vi.mocked(window.open).mockImplementation(() => { throw new Error('Missing protocol handler') })
    await render()
    await wrapper.get('[aria-label="导入到 CCS"]').trigger('click')
    expect(wrapper.get('[role="alert"]').text()).toContain('未能打开 CCS')
    expect(wrapper.text()).not.toContain(key.key)
  })

  it('requires confirmation for deletion and allows cancellation', async () => {
    await render()
    await wrapper.get('[aria-label="删除 Key"]').trigger('click')
    expect(api.delete).not.toHaveBeenCalled()
    expect(wrapper.get('[role="dialog"]').text()).toContain(key.name)
    await wrapper.get('[role="dialog"]').findAll('button')[0].trigger('click')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(api.delete).not.toHaveBeenCalled()
    await wrapper.get('[aria-label="删除 Key"]').trigger('click')
    api.list.mockResolvedValue({ items: [] })
    await wrapper.get('[role="dialog"]').findAll('button')[1].trigger('click')
    await flushPromises()
    expect(api.delete).toHaveBeenCalledWith(key.id)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-test="package-key-row"]')).toHaveLength(0)
  })

  it('blocks repeated delete requests while the first request is running', async () => {
    let finish!: () => void
    api.delete.mockReturnValue(new Promise<void>(resolve => { finish = resolve }))
    await render()
    await wrapper.get('[aria-label="删除 Key"]').trigger('click')
    const buttons = wrapper.get('[role="dialog"]').findAll('button')
    await buttons[1].trigger('click')
    await buttons[1].trigger('click')
    expect(api.delete).toHaveBeenCalledTimes(1)
    expect(buttons[0].attributes('disabled')).toBeDefined()
    expect(buttons[1].attributes('disabled')).toBeDefined()
    finish()
    await flushPromises()
  })

  it('keeps a failed deletion in the confirmation dialog so it can be retried', async () => {
    api.delete.mockRejectedValueOnce(new Error('Delete unavailable'))
    await render()
    await wrapper.get('[aria-label="删除 Key"]').trigger('click')
    await wrapper.get('[role="dialog"]').findAll('button')[1].trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="dialog"] [role="alert"]').text()).toBe('Delete unavailable')
    await wrapper.get('[role="dialog"]').findAll('button')[1].trigger('click')
    await flushPromises()
    expect(api.delete).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('preserves creation bound to an authorized package', async () => {
    await render()
    await wrapper.findAll('button').find(button => button.text() === '创建 Key')!.trigger('click')
    await wrapper.get('[role="dialog"] input').setValue('New package key')
    await wrapper.get('[role="dialog"] form').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith('New package key', 42, undefined, [], [], undefined, undefined)
  })
})
