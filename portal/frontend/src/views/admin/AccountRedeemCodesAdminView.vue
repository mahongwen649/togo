<template>
  <AppLayout>
    <div class="space-y-6">
      <header class="page-header flex flex-wrap items-end justify-between gap-4">
        <div><h1 class="page-title">账号卡密管理</h1><p class="page-description">生成和追踪账号系统专用卡密。卡密余额与中转站余额完全独立。</p></div>
        <button type="button" class="btn btn-primary" @click="showGenerate = true"><Icon name="plus" size="sm" class="mr-2" />生成卡密</button>
      </header>

      <div v-if="notice" class="rounded-lg border px-4 py-3 text-sm" :class="noticeError ? 'border-red-200 bg-red-50 text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300' : 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-900/20 dark:text-emerald-300'">{{ notice }}</div>

      <section class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="item in summary" :key="item.label" class="card p-5"><div class="text-sm text-gray-500 dark:text-dark-400">{{ item.label }}</div><div class="mt-2 text-2xl font-bold text-gray-900 dark:text-white">{{ item.value }}</div></div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 p-5 dark:border-dark-700">
          <div><h2 class="font-semibold text-gray-900 dark:text-white">卡密记录</h2><p class="mt-1 text-xs text-gray-500 dark:text-dark-400">列表只展示尾号，完整卡密仅在生成后显示一次。</p></div>
          <div class="flex flex-wrap gap-2"><input v-model="search" class="input w-44" placeholder="搜索批次或尾号" @keyup.enter="load" /><select v-model="status" class="input w-32" @change="load"><option value="">全部状态</option><option value="active">未兑换</option><option value="redeemed">已兑换</option><option value="partially_used">部分使用</option><option value="used">已用完</option><option value="expired">已过期</option><option value="disabled">已停用</option></select><button type="button" class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" /></button></div>
        </div>
        <div v-if="loading" class="p-10 text-center text-sm text-gray-500">正在加载卡密记录...</div>
        <div v-else-if="items.length === 0" class="p-10 text-center text-sm text-gray-500">暂无卡密记录</div>
        <div v-else class="overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">批次</th><th class="px-5 py-3">卡密</th><th class="px-5 py-3">面值 / 剩余</th><th class="px-5 py-3">状态</th><th class="px-5 py-3">兑换用户</th><th class="px-5 py-3">兑换时间</th><th class="px-5 py-3 text-right">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="item in items" :key="item.id"><td class="px-5 py-4"><div class="font-medium text-gray-900 dark:text-white">{{ item.batch_name || `批次 #${item.batch_id}` }}</div><div class="mt-1 text-xs text-gray-500">#{{ item.batch_id }}</div></td><td class="px-5 py-4 font-mono text-xs text-gray-600 dark:text-dark-300">****-{{ item.code_suffix }}</td><td class="px-5 py-4"><div>¥{{ item.amount.toFixed(2) }}</div><div class="mt-1 text-xs text-gray-500">剩余 ¥{{ item.remaining_amount.toFixed(2) }}</div></td><td class="px-5 py-4"><span class="badge" :class="statusClass(item.status)">{{ statusText(item.status) }}</span></td><td class="px-5 py-4 text-gray-600 dark:text-dark-300">{{ item.redeemed_user_id ? `用户 #${item.redeemed_user_id}` : '-' }}</td><td class="px-5 py-4 text-gray-500 dark:text-dark-400">{{ formatDate(item.redeemed_at) }}</td><td class="px-5 py-4 text-right"><button v-if="item.status === 'active' || item.status === 'expired'" type="button" class="btn btn-secondary btn-sm" :disabled="busy === item.id" @click="disable(item)">停用</button><span v-else class="text-xs text-gray-400">-</span></td></tr></tbody></table></div>
        <div v-if="pages > 1" class="flex items-center justify-between border-t border-gray-200 px-5 py-3 text-sm dark:border-dark-700"><span class="text-gray-500">共 {{ total }} 条</span><div class="flex gap-2"><button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="page--; load()">上一页</button><span class="px-2 py-1 text-gray-500">{{ page }} / {{ pages }}</span><button class="btn btn-secondary btn-sm" :disabled="page >= pages" @click="page++; load()">下一页</button></div></div>
      </section>

      <BaseDialog :show="showGenerate" title="生成账号系统卡密" @close="!saving && (showGenerate = false)">
        <form id="account-redeem-form" class="space-y-4" @submit.prevent="generate">
          <p class="text-sm text-gray-500 dark:text-dark-400">生成后完整卡密只在本次结果中显示，请及时复制保存。</p>
          <div class="grid gap-3 sm:grid-cols-2"><label class="block space-y-1 text-sm"><span>单张面值（元）</span><input v-model.number="form.amount" class="input w-full" type="number" min="0.01" step="0.01" required /></label><label class="block space-y-1 text-sm"><span>生成数量</span><input v-model.number="form.count" class="input w-full" type="number" min="1" max="5000" required /></label></div>
          <label class="block space-y-1 text-sm"><span>批次名称</span><input v-model="form.name" class="input w-full" placeholder="例如：2026年9月账号余额卡密" maxlength="100" /></label>
          <div class="grid gap-3 sm:grid-cols-2"><label class="block space-y-1 text-sm"><span>有效期（天，0 表示不过期）</span><input v-model.number="form.expires_in_days" class="input w-full" type="number" min="0" max="3650" /></label><label class="block space-y-1 text-sm"><span>备注</span><input v-model="form.note" class="input w-full" placeholder="内部备注" maxlength="200" /></label></div>
          <p v-if="formError" class="text-sm text-red-600">{{ formError }}</p>
        </form>
        <template #footer><button type="button" class="btn btn-secondary" :disabled="saving" @click="showGenerate = false">取消</button><button form="account-redeem-form" type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '生成中...' : '生成卡密' }}</button></template>
      </BaseDialog>

      <BaseDialog :show="generated.length > 0" title="卡密生成成功" width="wide" @close="generated = []">
        <div class="space-y-4"><p class="text-sm text-amber-700 dark:text-amber-300">请立即复制并保存以下完整卡密，关闭后后台只保留尾号用于追踪，无法再次查看完整内容。</p><textarea readonly class="input min-h-64 w-full font-mono text-sm" :value="generated.map(item => item.code).join('\n')" /><button type="button" class="btn btn-primary" @click="copyGenerated">复制全部卡密</button></div>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { accountRedeemAdminAPI, type AccountRedeemAdminCode } from '@/api/accountBusiness'

const items = ref<AccountRedeemAdminCode[]>([])
const generated = ref<AccountRedeemAdminCode[]>([])
const loading = ref(false)
const saving = ref(false)
const busy = ref<number | null>(null)
const showGenerate = ref(false)
const notice = ref('')
const noticeError = ref(false)
const formError = ref('')
const page = ref(1)
const pages = ref(1)
const total = ref(0)
const search = ref('')
const status = ref('')
const form = reactive({ name: '', amount: 100, count: 10, expires_in_days: 0, note: '' })

const summary = computed(() => [
  { label: '当前列表', value: `${total.value} 张` },
  { label: '未兑换', value: `${items.value.filter(item => item.status === 'active').length} 张` },
  { label: '部分使用', value: `${items.value.filter(item => item.status === 'partially_used').length} 张` },
  { label: '已用完', value: `${items.value.filter(item => item.status === 'used').length} 张` },
])

function statusText(value: string) { return ({ active: '未兑换', redeemed: '已兑换', partially_used: '部分使用', used: '已用完', expired: '已过期', disabled: '已停用' }[value] || value) }
function statusClass(value: string) { return value === 'active' || value === 'redeemed' ? 'badge-success' : value === 'partially_used' ? 'badge-warning' : value === 'used' ? 'badge-gray' : 'badge-danger' }
function formatDate(value: string | null) { return value ? new Date(value).toLocaleString('zh-CN') : '-' }

async function load() {
  loading.value = true
  try {
    const result = await accountRedeemAdminAPI.list({ page: page.value, page_size: 20, status: status.value || undefined, search: search.value || undefined })
    items.value = result.items.map(item => ({ ...item, amount: Number(item.amount || 0), remaining_amount: Number(item.remaining_amount || 0) }))
    pages.value = result.pages || 1
    total.value = result.total || 0
  } catch (err: any) { noticeError.value = true; notice.value = err?.message || '卡密记录加载失败' } finally { loading.value = false }
}

async function generate() {
  saving.value = true; formError.value = ''; notice.value = ''
  try {
    const result = await accountRedeemAdminAPI.generate(form)
    generated.value = result.codes || []
    showGenerate.value = false
    noticeError.value = false
    notice.value = `已生成 ${result.count} 张账号系统卡密`
    await load()
  } catch (err: any) { formError.value = err?.message || '卡密生成失败' } finally { saving.value = false }
}

async function disable(item: AccountRedeemAdminCode) {
  busy.value = item.id
  try { await accountRedeemAdminAPI.disable(item.id); noticeError.value = false; notice.value = '卡密已停用'; await load() } catch (err: any) { noticeError.value = true; notice.value = err?.message || '卡密停用失败' } finally { busy.value = null }
}

async function copyGenerated() {
  await navigator.clipboard.writeText(generated.value.map(item => item.code || '').join('\n'))
  noticeError.value = false; notice.value = '卡密已复制到剪贴板'
}

onMounted(() => { void load() })
</script>
