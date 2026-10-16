import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ExclusiveAccountUsage from './ExclusiveAccountUsage.vue'
const refreshUsage = vi.hoisted(() => vi.fn())
vi.mock('@/api/exclusiveChannels', () => ({ exclusiveChannels: { refreshUsage } }))
const account = { id: 15, name: '账号 01', available: true, five_hour_remaining: 100, weekly_remaining: 53, weekly_resets_at: '2026-09-20T13:00:00Z', observed_at: '2026-09-16T07:00:00Z' }
afterEach(() => vi.clearAllMocks())
describe('Channel usage windows', () => {
  it('shows Core utilization, remaining quota and reset time', () => {
    const w = mount(ExclusiveAccountUsage, { props: { accounts: [account], channelId: 6 } })
    expect(w.text()).toContain('已用 47%')
    expect(w.text()).toContain('剩余 53%')
    expect(w.text()).toContain('2026/9/20')
    expect(w.findAll('progress')[1].attributes('value')).toBe('47')
    w.unmount()
  })
  it('refreshes only the selected channel account and preserves quota on failure', async () => {
    refreshUsage.mockRejectedValueOnce(new Error('Core 查询失败')).mockResolvedValueOnce({ ...account, weekly_remaining: 50 })
    const w = mount(ExclusiveAccountUsage, { props: { accounts: [account], channelId: 6 } })
    await w.get('button').trigger('click'); await flushPromises()
    expect(refreshUsage).toHaveBeenCalledWith(6, 15)
    expect(w.get('[role="alert"]').text()).toBe('Core 查询失败')
    expect(w.text()).toContain('剩余 53%')
    await w.get('button').trigger('click'); await flushPromises()
    expect(w.text()).toContain('剩余 50%')
    expect(w.emitted('refreshed')).toHaveLength(1)
    w.unmount()
  })
})
