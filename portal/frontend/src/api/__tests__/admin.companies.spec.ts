import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, del } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    put,
    delete: del,
  },
}))

import companiesAPI from '@/api/admin/companies'

describe('admin companies api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    put.mockReset()
    del.mockReset()
  })

  it('creates a company through the company endpoint', async () => {
    post.mockResolvedValue({ data: { id: 9, name: 'Acme', status: 'active' } })

    const result = await companiesAPI.create({ name: 'Acme', status: 'active' })

    expect(post).toHaveBeenCalledWith('/admin/companies', { name: 'Acme', status: 'active' })
    expect(result.name).toBe('Acme')
  })

  it('updates a company through the company endpoint', async () => {
    put.mockResolvedValue({ data: { id: 7, name: 'Acme China', status: 'disabled' } })

    await companiesAPI.update(7, { name: 'Acme China', status: 'disabled' })

    expect(put).toHaveBeenCalledWith('/admin/companies/7', { name: 'Acme China', status: 'disabled' })
  })

  it('deactivates a company through the company endpoint', async () => {
    del.mockResolvedValue({ data: { id: 7, name: 'Acme', status: 'disabled' } })

    await companiesAPI.deactivate(7)

    expect(del).toHaveBeenCalledWith('/admin/companies/7')
  })

  it('assigns a user to a company through the company endpoint', async () => {
    post.mockResolvedValue({ data: { company_id: 7, user_id: 42, updated_api_keys: 2 } })

    await companiesAPI.assignMember(7, { user_id: 42 })

    expect(post).toHaveBeenCalledWith('/admin/companies/7/members', { user_id: 42 })
  })

  it('removes a user from a company through the company endpoint', async () => {
    del.mockResolvedValue({ data: { company_id: 7, user_id: 42, updated_api_keys: 1 } })

    await companiesAPI.removeMember(7, 42)

    expect(del).toHaveBeenCalledWith('/admin/companies/7/members/42')
  })

  it('loads company members with pagination params', async () => {
    get.mockResolvedValue({ data: { items: [], total: 0 } })

    await companiesAPI.getMembers(7, { page: 2, page_size: 50 })

    expect(get).toHaveBeenCalledWith('/admin/companies/7/members', {
      params: { page: 2, page_size: 50 },
    })
  })

  it('loads company managers', async () => {
    get.mockResolvedValue({ data: [{ company_id: 7, user_id: 42, email: 'manager@example.com' }] })

    const result = await companiesAPI.getManagers(7)

    expect(get).toHaveBeenCalledWith('/admin/companies/7/managers')
    expect(result[0].email).toBe('manager@example.com')
  })

  it('adds a company manager', async () => {
    post.mockResolvedValue({ data: { company_id: 7, user_id: 42 } })

    await companiesAPI.addManager(7, { user_id: 42 })

    expect(post).toHaveBeenCalledWith('/admin/companies/7/managers', { user_id: 42 })
  })

  it('removes a company manager', async () => {
    del.mockResolvedValue({ data: { company_id: 7, user_id: 42 } })

    await companiesAPI.removeManager(7, 42)

    expect(del).toHaveBeenCalledWith('/admin/companies/7/managers/42')
  })

  it('allocates member balance through the company endpoint', async () => {
    post.mockResolvedValue({ data: { id: 42, balance: 35 } })

    await companiesAPI.allocateBalance(7, 42, { amount: 25, notes: 'July budget' })

    expect(post).toHaveBeenCalledWith('/admin/companies/7/members/42/balance', {
      amount: 25,
      notes: 'July budget',
    })
  })
})
