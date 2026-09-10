import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RechargesView from '../RechargesView.vue'

const { listRecharges } = vi.hoisted(() => ({
  listRecharges: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    recharges: { list: listRecharges },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn() }),
}))

vi.mock('@/composables/useDeviceMode', () => ({
  useDeviceMode: () => ({ isMobileH5: { value: false } }),
}))

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div data-test="columns">{{ columns.map(column => column.key).join(',') }}</div>
      <div v-for="row in data" :key="row.id">
        <span data-test="row-key">{{ row.user_id ?? row.id }}</span>
        <slot name="cell-current_balance" :row="row" :value="row.current_balance" />
        <slot name="cell-total_recharged" :row="row" :value="row.total_recharged" />
      </div>
    </div>
  `,
}

describe('admin RechargesView', () => {
  beforeEach(() => {
    listRecharges.mockReset()
    listRecharges.mockResolvedValue({
      items: [{
        id: 121,
        user_id: 165,
        user_name: 'test-user',
        user_email: 'test@example.com',
        current_balance: 12.34,
        total_recharged: 125.5,
        amount: 5.03,
        pay_amount: 5.03,
        payment_type: 'alipay',
        status: 'COMPLETED',
        order_type: 'recharge',
        created_at: '2026-08-07 02:17:05',
        effective_time: '2026-08-07 02:17:05',
      }],
      total: 1,
      page: 1,
      page_size: 50,
      pages: 1,
      summary: {
        total_amount: 5.03,
        total_pay_amount: 5.03,
        all_time_total_recharged: 204,
        recharge_users_balance: 83.75,
        order_count: 1,
        user_count: 1,
        average_pay_amount: 5.03,
        max_pay_amount: 5.03,
        latest: null,
        start_time: '',
        end_time: '',
        granularity: 'hour',
      },
      users: [],
      users_total: 0,
      users_pages: 0,
      timeseries: [],
      timeseries_total: 0,
      timeseries_pages: 0,
    })
  })

  it('shows the recharging user current balance in the order details', async () => {
    const wrapper = mount(RechargesView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          SearchInput: true,
          Input: true,
          Select: true,
          DashboardStatCard: {
            props: ['label', 'value'],
            template: '<div data-test="stat-card">{{ label }}: {{ value }}</div>',
          },
          DataTable: DataTableStub,
          Pagination: true,
        },
      },
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text().split(',')
    expect(columns.slice(0, 5)).toEqual(['id', 'user', 'current_balance', 'total_recharged', 'pay_amount'])
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('12.34')
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('125.50')
    expect(wrapper.text()).toContain('累计总充值: 204.00')
    expect(wrapper.text()).toContain('充值用户总余额: 83.75')
  })

  it('does not present missing fields from an older backend as real zero values', async () => {
    const payload = await listRecharges()
    delete payload.items[0].total_recharged
    delete payload.summary.all_time_total_recharged
    delete payload.summary.recharge_users_balance
    listRecharges.mockResolvedValue(payload)

    const wrapper = mount(RechargesView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          SearchInput: true,
          Input: true,
          Select: true,
          DashboardStatCard: {
            props: ['label', 'value'],
            template: '<div data-test="stat-card">{{ label }}: {{ value }}</div>',
          },
          DataTable: DataTableStub,
          Pagination: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('累计总充值: -')
    expect(wrapper.text()).toContain('充值用户总余额: -')
    expect(wrapper.get('[data-test="data-table"]').text()).toContain('12.34-')
  })

  it('reloads the selected user statistics page and uses its own total', async () => {
    const initialPayload = await listRecharges()
    listRecharges.mockImplementation(async (filters) => ({
      ...initialPayload,
      page: filters.page,
      users: [{
        user_id: filters.page === 2 ? 202 : 101,
        user_name: filters.page === 2 ? 'second-page' : 'first-page',
        user_email: '',
        current_balance: 1,
        total_amount: 10,
        total_pay_amount: 10,
        order_count: 1,
        average_pay_amount: 10,
        max_pay_amount: 10,
        latest_time: '2026-08-28 00:00:00',
      }],
      users_total: 120,
      users_pages: 3,
    }))

    const wrapper = mount(RechargesView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          DateRangePicker: true,
          SearchInput: true,
          Input: true,
          Select: true,
          DashboardStatCard: true,
          DataTable: DataTableStub,
          Pagination: {
            props: ['page', 'total', 'pageSize'],
            emits: ['update:page', 'update:pageSize'],
            template: '<button data-test="next-page" :data-total="total" @click="$emit(\'update:page\', 2)">next</button>',
          },
        },
      },
    })
    await flushPromises()

    const usersTab = wrapper.findAll('button').find((button) => button.text() === '用户统计')
    expect(usersTab).toBeDefined()
    await usersTab!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="next-page"]').attributes('data-total')).toBe('120')
    expect(wrapper.get('[data-test="row-key"]').text()).toBe('101')

    await wrapper.get('[data-test="next-page"]').trigger('click')
    await flushPromises()
    expect(listRecharges).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2, page_size: 50 }))
    expect(wrapper.get('[data-test="row-key"]').text()).toBe('202')
  })
})
