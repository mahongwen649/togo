import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../../../../../')
const read = (path: string) => readFileSync(resolve(root, path), 'utf8')

describe('Account product configuration', () => {
  it('supports rental and account modes with custom rental validity', () => {
    const adminView = read('portal/frontend/src/views/admin/AccountMarketAdminView.vue')
    expect(adminView).toContain('value="rental"')
    expect(adminView).toContain('value="purchase"')
    expect(adminView).toContain('v-model.number="form.validity_days"')
    expect(adminView).toContain('list="account-product-validity-presets"')
  })

  it('uses an uploaded image instead of an editable image URL', () => {
    const adminView = read('portal/frontend/src/views/admin/AccountMarketAdminView.vue')
    const customerView = read('portal/frontend/src/views/user/AccountMarketView.vue')
    expect(adminView).toContain('type="file"')
    expect(adminView).toContain('handleProductImageChange')
    expect(adminView).toContain('class="h-full w-full object-contain"')
    expect(adminView).toContain('class="aspect-square w-full"')
    expect(customerView).toContain('class="aspect-square w-full bg-gray-100 dark:bg-dark-800"')
    expect(customerView).toContain('class="h-full w-full object-contain"')
    expect(customerView).not.toContain('class="h-full w-full object-cover"')
    expect(adminView).not.toContain('广告图地址')
    expect(adminView).not.toContain('placeholder="广告图 URL"')
  })

  it('limits auto-renew controls and expiry prompts to 30-day rentals', () => {
    const ordersView = read('portal/frontend/src/views/user/AccountPackageOrdersView.vue')
    const backend = read('portal/backend/internal/httpapi/account_market.go')
    const worker = read('portal/backend/internal/httpapi/account_auto_renew.go')
    expect(ordersView).toContain("order.mode === 'rental' && order.validity_days === 30")
    expect(backend).toContain("mode='rental' AND validity_days=30")
    expect(worker).toContain("mode='rental' AND validity_days=30")
  })
})
