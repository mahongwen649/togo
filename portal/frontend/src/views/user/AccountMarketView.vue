<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header flex flex-wrap items-end justify-between gap-4">
        <h1 class="page-title">购买套餐</h1>
        <RouterLink to="/account-business/orders" class="btn btn-secondary">查看我的订单</RouterLink>
      </header>

      <section class="flex flex-wrap items-center justify-between gap-4 border-y border-gray-200 py-5 dark:border-dark-600">
        <div>
          <div class="text-sm text-gray-500 dark:text-dark-400">账号套餐余额</div>
          <div class="mt-1 text-2xl font-bold text-primary-600 dark:text-primary-400">{{ walletBalance === null ? '--' : money(walletBalance) }}</div>
        </div>
        <div class="flex flex-wrap gap-2">
          <RouterLink to="/recharge" class="btn btn-primary">余额充值</RouterLink>
          <RouterLink to="/account-business/balance/redeem" class="btn btn-secondary">卡密兑换</RouterLink>
        </div>
      </section>
      <div v-if="error" role="alert" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">{{ error }}</div>
      <button v-if="lastOrderId" type="button" class="btn btn-secondary" @click="showOrder = true">查看刚刚购买的订单 #{{ lastOrderId }}</button>
      <div v-if="loading" class="py-10 text-center text-sm text-gray-500">正在加载可购买套餐...</div>
      <div v-else-if="products.length === 0" class="py-10 text-center text-sm text-gray-500">暂无有库存的上架套餐</div>
      <div v-else class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <article v-for="product in products" :key="product.id" class="card min-w-0 overflow-hidden">
          <div class="aspect-square w-full bg-gray-100 dark:bg-dark-800">
            <img v-if="product.image_url" :src="product.image_url" :alt="product.name" class="h-full w-full object-contain" />
            <div v-else class="flex h-full items-center justify-center text-primary-500"><Icon name="server" size="xl" /></div>
          </div>
          <div class="space-y-4 p-5">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <h2 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ product.name }}</h2>
              <span class="badge badge-success">{{ product.mode === 'rental' ? `租号 ${product.validity_days} 天` : '账号' }}</span>
            </div>
            <p class="min-h-10 break-words text-sm text-gray-500 dark:text-dark-400">{{ product.description || '管理员暂未填写套餐说明' }}</p>
            <p v-if="product.purchase_notice" class="break-words text-xs text-gray-600 dark:text-dark-300">{{ product.purchase_notice }}</p>
            <div class="flex flex-wrap items-end justify-between gap-2">
              <div><span class="text-2xl font-bold text-primary-600 dark:text-primary-400">{{ money(product.unit_price) }}</span><span class="ml-1 text-xs text-gray-500">/ 个</span></div>
              <span class="text-xs text-gray-500">剩余 {{ product.stock_remaining }} 个</span>
            </div>
            <div class="rounded-lg border border-primary-200 bg-primary-50/60 p-3 dark:border-primary-900/60 dark:bg-primary-950/20">
              <div class="mb-2 flex items-center justify-between gap-3">
                <label class="text-sm font-semibold text-gray-900 dark:text-white" :for="`product-quantity-${product.id}`">购买数量</label>
                <span class="text-xs text-gray-500 dark:text-dark-400">每个账号 {{ money(product.unit_price) }}</span>
              </div>
              <div class="flex items-center gap-3">
                <div class="flex items-center rounded-lg border border-gray-300 bg-white dark:border-dark-600 dark:bg-dark-800">
                  <button type="button" class="flex h-10 w-10 items-center justify-center text-lg text-gray-600 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40 dark:text-dark-200 dark:hover:bg-dark-700" :disabled="quantityFor(product) <= 1" aria-label="减少购买数量" @click="changeQuantity(product, -1)">−</button>
                  <input :id="`product-quantity-${product.id}`" v-model.number="quantities[product.id]" type="number" min="1" step="1" :max="product.stock_remaining" class="h-10 w-14 border-x border-gray-200 bg-transparent text-center font-semibold outline-none dark:border-dark-600" />
                  <button type="button" class="flex h-10 w-10 items-center justify-center text-lg text-gray-600 hover:bg-gray-100 disabled:cursor-not-allowed disabled:opacity-40 dark:text-dark-200 dark:hover:bg-dark-700" :disabled="quantityFor(product) >= product.stock_remaining" aria-label="增加购买数量" @click="changeQuantity(product, 1)">+</button>
                </div>
                <button type="button" class="btn btn-primary min-w-0 flex-1" :disabled="submitting !== null" @click="requestOrder(product)">{{ submitting === product.id ? '下单中...' : '立即下单' }}</button>
              </div>
              <p class="mt-2 text-xs text-gray-600 dark:text-dark-300">选择需要购买的账号数量，最多 {{ product.stock_remaining }} 个；下单后将从账号套餐余额中扣除 {{ money(product.unit_price * quantityFor(product)) }}。</p>
            </div>
            <div class="flex justify-between text-sm font-semibold text-gray-700 dark:text-dark-200"><span>共 {{ quantityFor(product) }} 个账号</span><span>合计 {{ money(product.unit_price * quantityFor(product)) }}</span></div>
          </div>
        </article>
      </div>
    </div>
    <BaseDialog :show="showOrder" title="套餐下单成功" @close="showOrder = false">
      <div class="mb-5 flex items-center justify-between gap-3 border-b border-gray-200 pb-4 dark:border-dark-600">
        <span class="font-mono text-lg font-semibold">订单 #{{ lastOrderId }}</span>
        <span class="badge badge-success">下单成功</span>
      </div>
      <p v-if="detailLoading" role="status" class="py-4 text-sm text-gray-500">正在加载订单明细...</p>
      <div v-else-if="detailError" role="alert" class="space-y-3 text-sm">
        <p>{{ detailError }}</p>
        <button type="button" class="btn btn-secondary" @click="loadOrderDetail">重新加载明细</button>
      </div>
      <dl v-else-if="orderDetail" class="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
        <div class="min-w-0"><dt class="text-gray-500">套餐名称</dt><dd class="mt-1 break-words font-medium">{{ orderDetail.product_name }}</dd></div>
        <div><dt class="text-gray-500">订单状态</dt><dd class="mt-1 font-medium">{{ statusLabel(orderDetail.status) }}</dd></div>
        <div><dt class="text-gray-500">业务类型</dt><dd class="mt-1">{{ orderDetail.mode === 'rental' ? `租号 · ${orderDetail.validity_days} 天` : '账号' }}</dd></div>
        <div><dt class="text-gray-500">购买数量</dt><dd class="mt-1">{{ orderDetail.quantity }} 个账号</dd></div>
        <div><dt class="text-gray-500">套餐单价</dt><dd class="mt-1">{{ money(orderDetail.unit_price) }}</dd></div>
        <div><dt class="text-gray-500">实付金额</dt><dd class="mt-1 text-xl font-semibold text-primary-600">{{ money(orderDetail.total_price) }}</dd></div>
        <div><dt class="text-gray-500">付款方式</dt><dd class="mt-1">账号套餐余额</dd></div>
        <div><dt class="text-gray-500">下单时间</dt><dd class="mt-1">{{ formatDate(orderDetail.created_at) }}</dd></div>
        <div><dt class="text-gray-500">到期时间</dt><dd class="mt-1">{{ orderDetail.expires_at ? formatDate(orderDetail.expires_at) : '待分配账号后生效' }}</dd></div>
        <div v-if="supportsAutoRenew(orderDetail)"><dt class="text-gray-500">自动续订</dt><dd class="mt-1">{{ orderDetail.auto_renew ? '已开启' : '未开启' }}</dd></div>
        <div class="min-w-0"><dt class="text-gray-500">已分配账号</dt><dd class="mt-1 break-words">{{ orderDetail.core_account_ids?.length ? orderDetail.core_account_ids.map(id => `#${id}`).join(', ') : '等待管理员分配' }}</dd></div>
      </dl>
      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showOrder = false">继续选购</button>
        <RouterLink to="/account-business/orders" class="btn btn-primary">查看我的订单</RouterLink>
      </template>
    </BaseDialog>
    <BaseDialog :show="!!pendingProduct" title="确认立即下单" @close="submitting === null && (pendingProduct = null)">
      <div v-if="pendingProduct" class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-dark-300">请确认以下订单信息。确认后会立即扣除账号套餐余额。</p>
        <dl class="grid grid-cols-2 gap-4 rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800">
          <div><dt class="text-gray-500">套餐</dt><dd class="mt-1 break-words font-semibold">{{ pendingProduct.name }}</dd></div>
          <div><dt class="text-gray-500">业务类型</dt><dd class="mt-1 font-semibold">{{ pendingProduct.mode === 'rental' ? `租号 · ${pendingProduct.validity_days} 天` : '账号' }}</dd></div>
          <div><dt class="text-gray-500">购买数量</dt><dd class="mt-1 font-semibold">{{ quantityFor(pendingProduct) }} 个账号</dd></div>
          <div><dt class="text-gray-500">单价</dt><dd class="mt-1">{{ money(pendingProduct.unit_price) }} / 个</dd></div>
          <div><dt class="text-gray-500">应付金额</dt><dd class="mt-1 text-lg font-bold text-primary-600">{{ money(pendingProduct.unit_price * quantityFor(pendingProduct)) }}</dd></div>
        </dl>
        <p class="text-xs text-amber-700 dark:text-amber-300">下单后订单会进入待处理状态，账号由管理员分配。</p>
      </div>
      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="submitting !== null" @click="pendingProduct = null">返回修改</button>
        <button type="button" class="btn btn-primary" :disabled="submitting !== null" @click="confirmOrder">{{ submitting !== null ? '下单中...' : '确认并下单' }}</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { apiClient } from '@/api/client'

interface Product {
  id: number
  name: string
  mode: 'rental' | 'purchase'
  validity_days: number
  unit_price: number
  stock_remaining: number
  image_url: string
  description: string
  purchase_notice: string
}
interface OrderDetail {
  id: number
  product_name: string
  mode: 'rental' | 'purchase'
  validity_days: number
  quantity: number
  unit_price: number
  total_price: number
  status: string
  created_at: string
  expires_at: string | null
  core_account_ids: number[]
  auto_renew: boolean
}
const products = ref<Product[]>([])
const quantities = reactive<Record<number, number>>({})
const walletBalance = ref<number | null>(null)
const loading = ref(true)
const submitting = ref<number | null>(null)
const error = ref('')
const lastOrderId = ref<number | null>(null)
const orderDetail = ref<OrderDetail | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const showOrder = ref(false)
const pendingProduct = ref<Product | null>(null)
const money = (value: number) => `¥${Number(value).toFixed(2)}`
const formatDate = (value: string) => new Date(value).toLocaleString('zh-CN')
const statusLabels: Record<string, string> = { pending: '待处理', processing: '处理中', completed: '已完成', expired: '已过期', cancelled: '已取消', refunded: '已退款' }
const statusLabel = (value: string) => statusLabels[value] || value
const supportsAutoRenew = (value: Pick<OrderDetail, 'mode' | 'validity_days'>) => value.mode === 'rental' && value.validity_days === 30

function quantityFor(product: Product) {
  const value = Number(quantities[product.id])
  return Math.max(1, Math.min(product.stock_remaining, Number.isFinite(value) ? Math.floor(value) : 1))
}

function changeQuantity(product: Product, delta: number) {
  quantities[product.id] = Math.max(1, Math.min(product.stock_remaining, quantityFor(product) + delta))
}

function requestOrder(product: Product) {
  if (submitting.value !== null) return
  pendingProduct.value = product
}

async function load() {
  loading.value = true
  try {
    const [productResponse, walletResponse] = await Promise.all([
      apiClient.get<{ items: Product[] }>('/account-market/products'),
      apiClient.get<{ balance: number }>('/account-market/wallet'),
    ])
    products.value = productResponse.data.items || []
    walletBalance.value = Number(walletResponse.data.balance)
    products.value.forEach(product => { if (!quantities[product.id]) quantities[product.id] = 1 })
  } catch {
    error.value = '套餐或余额加载失败，请刷新页面重试。'
  } finally {
    loading.value = false
  }
}

async function loadOrderDetail() {
  detailLoading.value = true
  detailError.value = ''
  try {
    const { data } = await apiClient.get<{ items: OrderDetail[] }>('/account-market/orders')
    orderDetail.value = data.items.find(order => order.id === lastOrderId.value) || null
    if (!orderDetail.value) throw new Error('Order detail not found')
  } catch {
    detailError.value = '订单已下单成功，明细暂时加载失败。请重试加载或前往“我的订单”，不要重复下单。'
  } finally {
    detailLoading.value = false
  }
}

async function createOrder(product: Product) {
  if (submitting.value !== null) return
  submitting.value = product.id
  error.value = ''
  try {
    const { data } = await apiClient.post<{ id: number; balance: number }>('/account-market/orders', { product_id: product.id, quantity: quantityFor(product) })
    lastOrderId.value = data.id
    walletBalance.value = Number(data.balance)
    orderDetail.value = null
    showOrder.value = true
    await Promise.all([loadOrderDetail(), load()])
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : '下单失败，请检查账号套餐余额和库存。'
  } finally {
    submitting.value = null
  }
}

async function confirmOrder() {
  if (!pendingProduct.value || submitting.value !== null) return
  const product = pendingProduct.value
  pendingProduct.value = null
  await createOrder(product)
}

onMounted(load)
</script>
