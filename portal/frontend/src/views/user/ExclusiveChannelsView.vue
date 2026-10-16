<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import UsageView from './UsageView.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ExclusiveAccountUsage from '@/components/exclusive/ExclusiveAccountUsage.vue'
import ExclusiveAccountBadge from '@/components/exclusive/ExclusiveAccountBadge.vue'
import ExclusivePricingForm from '@/components/exclusive/ExclusivePricingForm.vue'
import ExclusivePricingSummary from '@/components/exclusive/ExclusivePricingSummary.vue'
import { exclusiveChannels as api, durationOptions, type ChannelAccount, type ChannelEvent, type ChannelForm, type ChannelLease, type ChannelState, type ExclusiveChannel, type MyChannel } from '@/api/exclusiveChannels'
import { buildCcSwitchImportDeeplink } from '@/utils/ccswitchImport'
import type { PublicSettings } from '@/types'

const route = useRoute()
const appStore = useAppStore()
const admin = computed(() => route.path.startsWith('/admin/'))
const tab = ref<'lobby' | 'mine' | 'history'>('lobby')
const channels = ref<ExclusiveChannel[]>([])
const mine = ref<MyChannel>({ lease: null, channel: null, balance: 0, server_time: '' })
const events = ref<ChannelEvent[]>([])
const page = ref(1), pages = ref(1), total = ref(0)
const search = ref(''), filter = ref('all')
const loading = ref(false), busy = ref(false), error = ref(''), notice = ref(''), initialized = ref(false)
const modal = ref<'claim' | 'release' | 'edit' | 'delete' | 'test' | null>(null)
const target = ref<ExclusiveChannel | null>(null)
const duration = ref(1440)
const form = reactive<ChannelForm>({ name: '', account_ids: [], durations: [...durationOptions], cooldown_seconds: 60, rate_multiplier: 1, model_pricing: [], image_price: 0.05, image_rate_multiplier: 1 })
const candidates = ref<ChannelAccount[]>([]), selectedAccounts = ref<ChannelAccount[]>([])
const accountSearch = ref(''), accountPage = ref(1), accountPages = ref(1), accountLoading = ref(false)
const publicSettings = ref<PublicSettings | null>(null), settingsReady = ref(false)
const now = ref(Date.now())
let clockOffset = 0, generation = 0, accountGeneration = 0
let clock: ReturnType<typeof setInterval>, poll: ReturnType<typeof setTimeout> | undefined, importTimer: ReturnType<typeof setTimeout> | undefined
let stopped = false
const states: Record<ChannelState, string> = { provisioning: '发布同步中', available: '空闲', active: '使用中', releasing: '回收清理中', cooling: '冷却中', maintenance: '维护中', unpublished: '已下架', error: '同步未完成' }
const reasons: Record<string, string> = { manual: '主动归还', expired: '到期回收', idle: '30 分钟无消费回收', admin: '管理员回收' }
const usageView = ref<InstanceType<typeof UsageView> | null>(null)
async function refresh() { await load(); usageView.value?.refresh() }
const money = (n: number) => `¥${n.toFixed(2)}`
const stamp = (s?: string | null) => s ? new Date(s).toLocaleString('zh-CN', { hour12: false }) : '-'
const label = (n: number) => n < 60 ? `${n} 分钟` : `${n / 60} 小时`
const countdown = (date?: string | null) => {
  const seconds = Math.max(0, Math.ceil(((date ? Date.parse(date) : now.value) - now.value) / 1000))
  return [Math.floor(seconds / 3600), Math.floor(seconds % 3600 / 60), seconds % 60].map(n => String(n).padStart(2, '0')).join(':')
}
const idleAt = (l: ChannelLease) => new Date(Math.max(Date.parse(l.started_at), l.last_charge_at ? Date.parse(l.last_charge_at) : 0) + 1800000).toISOString()
const visible = computed(() => channels.value.filter(c => (filter.value === 'all' || c.state === filter.value) && c.name.toLowerCase().includes(search.value.toLowerCase())))
const accountChoices = computed(() => {
  const choices = new Map<number, ChannelAccount>()
  for (const account of [...(target.value?.accounts || []), ...(target.value?.pending_accounts || []), ...selectedAccounts.value, ...candidates.value]) choices.set(account.id, account)
  return [...choices.values()]
})
const healthy = (c: ExclusiveChannel) => c.accounts.filter(a => a.available).length
const canClaim = (c: ExclusiveChannel) => initialized.value && !error.value && !mine.value.lease && c.state === 'available' && c.pricing_synced !== false && mine.value.balance > 0 && healthy(c) > 0
const configurationEditingLocked = computed(() => !!target.value && ['provisioning', 'error'].includes(target.value.state))
const configurationDeferred = computed(() => !!target.value && (!!target.value.lease || ['active', 'releasing', 'cooling'].includes(target.value.state)))
const pricingLocked = computed(() => busy.value || configurationEditingLocked.value)
const accountEditingLocked = computed(() => busy.value || configurationEditingLocked.value)
function tiltCard(event: PointerEvent) {
  if (event.pointerType !== 'mouse' || !window.matchMedia('(hover:hover) and (prefers-reduced-motion:no-preference)').matches) return
  const card = event.currentTarget as HTMLElement
  const box = card.getBoundingClientRect()
  card.style.setProperty('--tilt-x', `${(0.5 - (event.clientY - box.top) / box.height) * 3}deg`)
  card.style.setProperty('--tilt-y', `${((event.clientX - box.left) / box.width - 0.5) * 3}deg`)
}
function resetTilt(event: PointerEvent) {
  const card = event.currentTarget as HTMLElement
  card.style.removeProperty('--tilt-x'); card.style.removeProperty('--tilt-y')
}
const message = (e: unknown) => {
  const value = e as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message || value.message || '操作失败，请稍后重试'
}
async function load() {
  const current = ++generation; loading.value = true
  try {
    const [list, own, audit] = await Promise.all([api.list(admin.value, page.value), api.mine(), admin.value ? api.events() : Promise.resolve(null)])
    if (current !== generation || stopped) return
    channels.value = list.items; pages.value = Math.max(1, list.pages); total.value = list.total; mine.value = own
    if (audit) events.value = audit
    clockOffset = Date.parse(own.server_time) - Date.now(); now.value = Date.now() + clockOffset
    error.value = ''; initialized.value = true
  } catch (e) { if (current === generation && !stopped) error.value = message(e) }
  finally { if (current === generation) loading.value = false }
}
async function polling() { await load(); if (!stopped) poll = setTimeout(polling, 10000) }
function open(kind: typeof modal.value, c?: ExclusiveChannel) { error.value = ''; target.value = c || null; duration.value = 1440; modal.value = kind }
async function mutate(action: () => Promise<unknown>, success: string) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action(); modal.value = null; notice.value = success; await load() }
  catch (e) { const failure = message(e); if (!modal.value) await load(); error.value = failure }
  finally { busy.value = false }
}
async function claim() { if (!target.value) return; await mutate(async () => { await api.claim(target.value!.id, duration.value, target.value!.rate_multiplier ?? 1, target.value!.pricing_revision); tab.value = 'mine' }, '通道已领取') }
async function release() { if (target.value) await mutate(() => api.release(target.value!.id, admin.value), '已停止接收新请求，等待回收清理') }
async function removeChannel() { if (target.value) await mutate(() => api.remove(target.value!.id), '通道已删除') }
async function edit(c?: ExclusiveChannel) {
  const availableAccounts = [...(c?.accounts || []), ...(c?.pending_accounts || [])]
  const pending = c?.pending_config
  const selectedIDs = pending?.account_ids || availableAccounts.map(a => a.id)
  Object.assign(form, { name: c?.name || '', account_ids: [...selectedIDs], durations: c ? [...c.durations] : [...durationOptions], cooldown_seconds: c?.cooldown_seconds ?? 60, rate_multiplier: pending?.rate_multiplier ?? c?.rate_multiplier ?? 1, model_pricing: (pending?.model_pricing || c?.model_pricing || []).map(p => ({ ...p })), image_price: pending?.image_price ?? c?.image_price ?? 0.05, image_rate_multiplier: pending?.image_rate_multiplier ?? c?.image_rate_multiplier ?? 1 })
  selectedAccounts.value = availableAccounts.filter(a => selectedIDs.includes(a.id)); candidates.value = []; accountSearch.value = ''; accountPage.value = 1
  open('edit', c); await loadAccounts()
}
async function loadAccounts() {
  const current = ++accountGeneration; accountLoading.value = true
  try { const result = await api.accounts(accountPage.value, accountSearch.value); if (current !== accountGeneration || stopped) return; candidates.value = result.items; accountPages.value = Math.max(1, result.pages) }
  catch (e) { if (current === accountGeneration) error.value = message(e) }
  finally { if (current === accountGeneration) accountLoading.value = false }
}
function toggleAccount(a: ChannelAccount, checked: boolean) {
  selectedAccounts.value = checked ? [...selectedAccounts.value, a] : selectedAccounts.value.filter(v => v.id !== a.id)
  form.account_ids = selectedAccounts.value.map(a => a.id)
}
async function save() { await mutate(async () => { const result = await api.save({ ...form, image_price: form.image_price ?? 0.05, image_rate_multiplier: form.image_rate_multiplier ?? 1, durations: [...form.durations] }, target.value?.id); if ('error' in result && typeof result.error === 'string') { modal.value = null; throw new Error(result.error) } }, '通道配置已保存') }
async function changeState(c: ExclusiveChannel, event: Event) {
  const input = event.target as HTMLSelectElement; const state = input.value; input.value = c.state
  await mutate(() => api.availability(c.id, state), '发布状态已更新')
}
async function copy(text: string) {
  try { await navigator.clipboard.writeText(text); appStore.showSuccess('复制成功') }
  catch { appStore.showError('无法访问剪贴板，请选择凭据文本复制') }
}
async function loadPublicSettings() {
  try { publicSettings.value = await appStore.fetchPublicSettings() }
  catch { publicSettings.value = null }
  finally { settingsReady.value = true }
}
function importToCcswitch() {
  const lease = mine.value.lease
  const channel = mine.value.channel
  if (!settingsReady.value || !lease?.base_url || !lease.key || !channel) return
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl: lease.base_url,
    platform: 'openai',
    clientType: 'claude',
    providerName: `${(publicSettings.value?.site_name || 'TogoAPI').trim() || 'TogoAPI'} · ${channel.name}`,
    apiKey: lease.key
  })
  const notInstalled = '未能打开 CCS，请确认已安装 CC-Switch 并注册导入协议。'
  clearTimeout(importTimer)
  try {
    window.open(deeplink, '_self')
    importTimer = setTimeout(() => { if (document.hasFocus()) appStore.showError(notInstalled) }, 100)
  } catch { appStore.showError(notInstalled) }
}
watch([admin, tab, page], () => { void load() })
onMounted(() => { clock = setInterval(() => { now.value = Date.now() + clockOffset }, 1000); void polling(); void loadPublicSettings() })
onUnmounted(() => { stopped = true; generation++; accountGeneration++; clearInterval(clock); if (poll) clearTimeout(poll); clearTimeout(importTimer) })
</script>

<template>
  <AppLayout>
    <div class="ex-page">
      <header class="ex-heading"><div><div class="ex-eyebrow">独享通道</div><h1>{{ admin ? '通道管理' : tab === 'mine' ? '我的通道' : tab === 'history' ? '使用记录' : '通道大厅' }}</h1></div><div class="ex-header-actions"><div v-if="!admin" class="ex-balance"><span>中转余额</span><strong>{{ initialized ? money(mine.balance) : '-' }}</strong></div><button class="btn btn-secondary" :disabled="loading" aria-label="刷新通道" title="刷新通道" @click="refresh"><Icon name="refresh" size="sm" /></button><button v-if="admin" class="btn btn-primary" :disabled="!initialized || busy" @click="edit()"><Icon name="plus" size="sm" />发布通道</button></div></header>
      <nav v-if="!admin" class="ex-tabs" aria-label="独享通道视图"><button v-for="item in [{ id: 'lobby', label: '通道大厅' }, { id: 'mine', label: '我的通道' }, { id: 'history', label: '使用记录' }] as const" :key="item.id" :class="{ selected: tab === item.id }" @click="tab = item.id">{{ item.label }}</button></nav>
      <div class="ex-feedback" aria-live="polite"><p v-if="error && !modal" role="alert" class="ex-error">{{ error }}</p><p v-else>{{ notice }}</p></div>
      <div v-if="!initialized && loading" class="ex-empty">正在加载通道...</div>
      <template v-else-if="admin">
        <div class="ex-table-scroll">
          <table><thead><tr><th>通道 / 分组</th><th>状态</th><th>当前使用者</th><th>账号</th><th>冷却时间</th><th>通道定价</th><th>操作</th></tr></thead><tbody>
            <tr v-for="c in channels" :key="c.id">
              <td><strong>{{ c.name }}</strong><small>Core 分组 #{{ c.core_group_id || '-' }}</small><small v-if="c.error" class="ex-error">{{ c.error }}</small></td>
              <td><span class="ex-status" :data-state="c.state">{{ states[c.state] }}</span><small v-if="c.cooldown_until">{{ countdown(c.cooldown_until) }}</small></td>
              <td>{{ c.lease ? `用户 #${c.lease.user_id}` : '未占用' }}<small v-if="c.lease">{{ c.lease.state === 'provisioning' ? '凭据同步中' : `${stamp(c.lease.expires_at)} 到期` }}</small></td>
              <td>{{ c.accounts.length }} 个</td><td>{{ c.cooldown_seconds }} 秒</td>
              <td class="ex-pricing-cell">
                <strong :class="{ 'ex-pending-rate': c.pending_config }">{{ c.pending_config?.rate_multiplier ?? c.rate_multiplier ?? 1 }}×</strong>
                <small v-if="c.pending_config" class="ex-pending-note">待生效 · 当前 {{ c.rate_multiplier ?? 1 }}×</small>
                <small v-if="c.pending_config">待生效图片 ¥{{ c.pending_config.image_price }} / 张 × {{ c.pending_config.image_rate_multiplier }}</small>
                <small>{{ c.pricing_synced === false ? '新价格待同步' : `${c.pending_config ? '当前' : '图片'} ¥${c.image_price ?? 0.05} / 张 × ${c.image_rate_multiplier ?? 1}` }}</small>
                <small>{{ c.pending_config ? `待生效文本：${c.pending_config.model_pricing.length ? `${c.pending_config.model_pricing.length} 个模型独立定价` : '默认价'}` : c.model_pricing?.length ? `${c.model_pricing.length} 个模型独立定价` : '文本默认价' }}</small>
              </td>
              <td><div class="ex-actions">
                <button class="btn btn-secondary" :disabled="busy" :aria-label="`编辑 ${c.name}`" :title="`编辑 ${c.name}`" @click="edit(c)"><Icon name="edit" size="sm" /></button>
                <button class="btn btn-secondary" :disabled="busy || !c.accounts.length" :aria-label="`测试 ${c.name} 账号`" :title="`测试 ${c.name} 账号`" @click="open('test', c)"><Icon name="beaker" size="sm" /></button>
                <button class="btn btn-secondary ex-delete" :disabled="busy || !!c.lease || !['available', 'maintenance', 'unpublished', 'error'].includes(c.state)" :aria-label="`删除 ${c.name}`" :title="`删除 ${c.name}`" @click="open('delete', c)"><Icon name="trash" size="sm" /></button>
                <button v-if="c.state === 'error' || c.state === 'provisioning'" class="btn btn-secondary" :disabled="busy" @click="mutate(() => api.retry(c.id), 'Core 配置已同步')">重试同步</button>
                <button v-else-if="c.lease" class="btn btn-secondary" :disabled="busy || c.lease.state !== 'active'" @click="open('release', c)">回收</button>
                <select v-else :value="c.state" :disabled="busy || c.state === 'cooling'" :aria-label="`${c.name} 发布状态`" @change="changeState(c, $event)"><option v-if="c.state === 'cooling'" value="cooling">冷却中</option><option value="available">开放</option><option value="maintenance">维护</option><option value="unpublished">下架</option></select>
              </div></td>
            </tr>
          </tbody></table>
        </div>
        <section class="ex-audit"><h2>操作记录</h2><div class="ex-table-scroll"><table><thead><tr><th>时间</th><th>通道</th><th>操作人</th><th>操作</th></tr></thead><tbody><tr v-for="e in events" :key="e.id"><td>{{ stamp(e.created_at) }}</td><td>{{ e.channel_name }}</td><td>{{ e.actor_id ? `用户 #${e.actor_id}` : '系统' }}</td><td>{{ e.action }}</td></tr></tbody></table></div></section>
      </template>
      <template v-else-if="tab === 'lobby'">
        <div v-if="mine.lease" class="ex-occupied"><span><strong>{{ mine.channel?.name }}</strong> · {{ mine.lease.state === 'active' ? `剩余 ${countdown(mine.lease.expires_at)}` : mine.lease.state === 'provisioning' ? '凭据同步未完成' : '回收清理中' }}</span><button class="btn btn-secondary" @click="tab = 'mine'">查看我的通道<Icon name="chevronRight" size="sm" /></button></div>
        <div class="ex-toolbar"><select v-model="filter" aria-label="通道状态"><option value="all">全部状态</option><option value="available">空闲</option><option value="active">使用中</option><option value="releasing">回收清理中</option><option value="cooling">冷却中</option></select><span class="ex-muted">共 {{ total }} 个通道</span><input v-model="search" type="search" aria-label="搜索当前页通道" placeholder="搜索当前页通道" /></div>
        <div class="ex-grid">
          <article v-for="(c, index) in visible" :key="c.id" class="ex-channel" :data-state="c.state" :style="{ '--entry-delay': `${Math.min(index, 5) * 45}ms` }" @pointermove="tiltCard" @pointerleave="resetTilt" @pointercancel="resetTilt">
            <div class="ex-card-top">
              <div class="ex-card-identity"><Icon name="server" size="lg" /><span class="ex-status" :data-state="c.state">{{ c.lease?.mine ? '我的通道' : states[c.state] }}</span></div>
              <div class="ex-rate"><span>通道倍率</span><strong>{{ c.rate_multiplier ?? 1 }}<small>×</small></strong></div>
            </div>
            <h2>{{ c.name }}</h2>
            <p class="ex-card-subtitle">OpenAI · 专属标准分组</p>
            <template v-if="c.lease">
              <div class="ex-countdown"><span>到期回收倒计时</span><strong>{{ countdown(c.lease.expires_at) }}</strong></div>
              <div v-if="c.lease.state === 'active'" class="ex-countdown ex-idle-countdown"><span>无消费回收倒计时</span><strong>{{ countdown(idleAt(c.lease)) }}</strong></div>
              <div v-else class="ex-warning">{{ c.lease.state === 'provisioning' ? '凭据同步中' : '等待在途请求结束并清理' }}</div>
            </template>
            <div class="ex-account-summary"><span>可用账号 <strong>{{ healthy(c) }}</strong> / {{ c.accounts.length }}</span><span>用量窗口</span></div>
            <ExclusiveAccountUsage :accounts="c.accounts" :channel-id="c.id" @refreshed="load" />
            <ExclusivePricingSummary :channel="c" />
            <div class="ex-card-footer"><span>中转额度计费</span><span>归还后冷却 {{ c.cooldown_seconds }} 秒</span></div>
            <button class="btn ex-claim" :class="canClaim(c) ? 'btn-primary' : 'btn-secondary'" :disabled="busy || loading || (!canClaim(c) && !c.lease?.mine)" @click="c.lease?.mine ? tab = 'mine' : open('claim', c)">{{ c.lease?.mine ? '查看我的通道' : c.state === 'cooling' ? `冷却 ${countdown(c.cooldown_until)}` : c.state !== 'available' ? states[c.state] : mine.lease ? '已有使用中的通道' : mine.balance <= 0 ? '中转余额不足' : c.pricing_synced === false ? '价格同步中' : '领取通道' }}<Icon v-if="canClaim(c) || c.lease?.mine" name="chevronRight" size="sm" /></button>
          </article>
        </div>
        <div v-if="!visible.length" class="ex-empty">暂无匹配的通道</div>
      </template>
      <template v-else-if="tab === 'mine'">
        <template v-if="mine.lease && mine.channel">
          <h2>{{ mine.channel.name }}</h2>
          <p class="ex-muted">中转额度计费 · 通道倍率 {{ mine.channel.rate_multiplier ?? 1 }}×</p>
          <ExclusivePricingSummary :channel="mine.channel" />
          <p v-if="mine.lease.error" class="ex-error">{{ mine.lease.error }}</p>
          <p v-if="mine.lease.uncertain" class="ex-warning-box">请求完成状态不确定，通道保持锁定，请联系管理员核查。</p>
          <section class="ex-lease-stats"><div><span>剩余使用时间</span><strong>{{ mine.lease.state === 'active' ? countdown(mine.lease.expires_at) : '等待处理' }}</strong><small>{{ stamp(mine.lease.expires_at) }} 到期</small><p v-if="mine.lease.state === 'active'">无消费回收倒计时 {{ countdown(idleAt(mine.lease)) }}</p><p v-else>{{ reasons[mine.lease.reason] }}</p></div><div>本次消费<strong>{{ money(mine.lease.cost) }}</strong></div><div>完成请求<strong>{{ mine.lease.requests }}</strong></div><div>Token 用量<strong>{{ mine.lease.tokens }}</strong></div></section>
          <div class="ex-workspace"><div class="ex-workspace-main">
          <section v-if="mine.lease.state === 'active'" class="ex-section"><div class="ex-section-heading"><h3>接入信息</h3><button v-if="!publicSettings?.hide_ccs_import_button" type="button" class="btn btn-secondary" :disabled="!settingsReady" aria-label="导入到 CCS" title="导入到 CCS" @click="importToCcswitch"><Icon name="upload" size="sm" />导入到 CCS</button></div><label v-for="field in [{ label: 'Base URL', value: mine.lease.base_url }, { label: 'API Key', value: mine.lease.key }]" :key="field.label" class="ex-credential"><span>{{ field.label }}</span><input :value="field.value" readonly :aria-label="field.label" /><button class="btn btn-secondary" :aria-label="`复制 ${field.label}`" :title="`复制 ${field.label}`" @click="copy(field.value || '')"><Icon name="copy" size="sm" /></button></label></section>
          <section v-if="mine.lease.inflight" class="ex-section"><h3>在途请求</h3><p>{{ mine.lease.inflight }} 个请求正在处理</p></section>
          <section class="ex-section"><h3>账号用量与降智测试</h3><ExclusiveAccountUsage :accounts="mine.channel.accounts" :channel-id="mine.channel.id" :can-test="mine.lease.state === 'active' && now < Date.parse(mine.lease.expires_at) && now < Date.parse(idleAt(mine.lease))" @refreshed="load" /></section>
          </div></div>
          <div class="ex-return">
            <template v-if="mine.lease.state === 'provisioning'">
              <button class="btn btn-secondary" :disabled="busy" @click="open('release', mine.channel)">取消领取</button>
              <button class="btn btn-primary" :disabled="busy" @click="mutate(() => api.claim(mine.channel!.id, 1440), '凭据同步完成')">重试领取</button>
            </template>
            <button v-else class="btn btn-secondary" :disabled="busy || mine.lease.state !== 'active'" @click="open('release', mine.channel)"><Icon name="swap" size="sm" />{{ mine.lease.state === 'active' ? '归还通道' : '回收清理中' }}</button>
          </div>
        </template>
        <div v-else class="ex-empty"><h2>暂无正在使用的通道</h2><button class="btn btn-primary" @click="tab = 'lobby'">前往通道大厅</button></div>
      </template>
      <UsageView v-else ref="usageView" embedded scope="exclusive" />
      <div v-if="(admin || tab === 'lobby') && pages > 1" class="ex-pagination"><button class="btn btn-secondary" :disabled="loading || page <= 1" aria-label="上一页" @click="page--"><Icon name="chevronLeft" size="sm" /></button><span>{{ page }} / {{ pages }}</span><button class="btn btn-secondary" :disabled="loading || page >= pages" aria-label="下一页" @click="page++"><Icon name="chevronRight" size="sm" /></button></div>
      <BaseDialog :show="modal !== null" :width="modal === 'test' ? 'extra-wide' : modal === 'edit' ? 'wide' : 'normal'" :title="modal === 'claim' ? '领取通道' : modal === 'release' ? '归还通道' : modal === 'delete' ? '删除通道' : modal === 'test' ? '账号降智测试' : target ? '编辑通道' : '发布通道'" :close-on-escape="!busy" :show-close-button="!busy" @close="modal = null">
        <div class="ex-dialog"><p v-if="error" role="alert" class="ex-error">{{ error }}</p>
          <template v-if="modal === 'test' && target"><h3>{{ target.name }}</h3><ExclusiveAccountUsage :accounts="target.accounts" :channel-id="target.id" :can-test="true" @refreshed="load" /></template>
          <template v-else-if="modal === 'claim' && target"><h3>{{ target.name }}</h3><label class="ex-field"><span>使用时长</span><select v-model.number="duration" aria-label="使用时长"><option v-for="d in target.durations" :key="d" :value="d">{{ label(d) }}{{ d === 1440 ? '（默认）' : '' }}</option></select></label><dl class="ex-facts"><div><dt>预计到期</dt><dd>{{ stamp(new Date(now + duration * 60000).toISOString()) }}</dd></div><div><dt>归还后冷却</dt><dd>{{ target.cooldown_seconds }} 秒</dd></div><div><dt>扣费账户</dt><dd>中转余额 · {{ money(mine.balance) }}</dd></div><div><dt>通道倍率</dt><dd>{{ target.rate_multiplier ?? 1 }}×</dd></div><div><dt>领取费用</dt><dd>¥0.00</dd></div></dl><ExclusivePricingSummary :channel="target" /><div class="ex-warning-box"><Icon name="exclamationTriangle" size="sm" /><p><strong>连续 30 分钟未产生消费将自动回收。</strong>产生消费后重新计时；回收仍需等待在途请求结束并完成清理。</p></div></template>
          <template v-else-if="modal === 'release' && target"><h3>{{ target.name }}</h3><p>归还后停止接收新请求，等待已开始的请求结束并清理，再进入 {{ target.cooldown_seconds }} 秒冷却。</p></template>
          <template v-else-if="modal === 'delete' && target"><h3>{{ target.name }}</h3><div class="ex-warning-box"><Icon name="exclamationTriangle" size="sm" /><p><strong>删除后无法恢复。</strong>绑定账号会从 Core 分组解绑，自动创建的 Core 分组、通道使用记录和测试页面将一并删除。</p></div></template>
          <form v-else-if="modal === 'edit'" id="exclusive-channel-form" @submit.prevent="save">
            <label class="ex-field"><span>通道名称</span><input v-model="form.name" required maxlength="40" aria-label="通道名称" /></label>
            <div class="ex-field"><span>{{ target ? '调整绑定账号' : '绑定账号' }}</span>
              <details class="ex-picker"><summary>{{ selectedAccounts.length ? `已选择 ${selectedAccounts.length} 个账号` : '请选择可用账号' }}<Icon name="chevronDown" size="sm" /></summary><div class="ex-picker-body">
                <div class="ex-actions"><input v-model="accountSearch" type="search" aria-label="搜索可用账号" placeholder="搜索账号" @keydown.enter.prevent="accountPage = 1; loadAccounts()" /><button type="button" class="btn btn-secondary" :disabled="accountLoading" aria-label="搜索账号" @click="accountPage = 1; loadAccounts()"><Icon name="search" size="sm" /></button></div>
                <p v-if="accountLoading">正在加载账号...</p>
                <label v-for="a in accountChoices" :key="a.id" class="ex-option"><input type="checkbox" :checked="form.account_ids?.includes(a.id)" :disabled="accountEditingLocked" :aria-label="a.name" @change="toggleAccount(a, ($event.target as HTMLInputElement).checked)" /><span>{{ a.name }}<small>#{{ a.id }}{{ target?.accounts.some(item => item.id === a.id) ? ' · 当前绑定' : ' · 可新增' }}</small><ExclusiveAccountBadge :account="a" /></span></label>
                <p v-if="!accountLoading && !accountChoices.length">暂无可用账号</p>
                <div v-if="accountPages > 1" class="ex-pagination"><button type="button" class="btn btn-secondary" :disabled="accountLoading || accountPage <= 1" aria-label="上一页账号" @click="accountPage--; loadAccounts()"><Icon name="chevronLeft" size="sm" /></button><span>{{ accountPage }} / {{ accountPages }}</span><button type="button" class="btn btn-secondary" :disabled="accountLoading || accountPage >= accountPages" aria-label="下一页账号" @click="accountPage++; loadAccounts()"><Icon name="chevronRight" size="sm" /></button></div>
              </div></details>
              <div class="ex-muted">{{ selectedAccounts.map(a => a.name).join('、') }}</div>
              <span v-if="configurationEditingLocked" class="ex-warning">Core 配置正在同步，请同步完成后再调整账号和定价。</span>
              <span v-else-if="configurationDeferred" class="ex-warning">当前通道正在使用、回收或冷却。账号和定价可以编辑，保存后将在当前使用结束并完成冷却后自动生效。</span>
              <span v-else-if="target" class="ex-warning">可取消勾选解绑，也可重新勾选或选择新账号；至少保留 1 个账号。</span>
            </div>
            <dl v-if="target" class="ex-facts"><div><dt>Core 分组</dt><dd>#{{ target.core_group_id || '-' }}</dd></div></dl>
            <label class="ex-field"><span>通道倍率</span><input v-model.number="form.rate_multiplier" type="number" min="0.001" step="0.001" required aria-label="通道倍率" :disabled="pricingLocked" /><span v-if="configurationDeferred" class="ex-warning">新倍率将在当前使用结束并完成冷却后生效</span></label>
            <ExclusivePricingForm :model-value="form.model_pricing || []" :image-price="form.image_price ?? null" :image-rate="form.image_rate_multiplier ?? null" :disabled="pricingLocked" @update:image-rate="form.image_rate_multiplier = $event" @update:model-value="form.model_pricing = $event" @update:image-price="form.image_price = $event" />
            <label class="ex-field"><span>归还后冷却时间（秒）</span><input v-model.number="form.cooldown_seconds" type="number" min="0" max="86400" step="1" required aria-label="归还后冷却时间（秒）" /></label><fieldset><legend>可选使用时长</legend><div class="ex-durations"><label v-for="d in durationOptions" :key="d"><input v-model="form.durations" type="checkbox" :value="d" :disabled="d === 1440" />{{ label(d) }}{{ d === 1440 ? '（默认）' : '' }}</label></div></fieldset>
          </form>
        </div>
        <template #footer><div class="ex-actions ex-dialog-footer"><button class="btn btn-secondary" :disabled="busy" @click="modal = null">{{ modal === 'test' ? '关闭' : '取消' }}</button><button v-if="modal === 'claim'" class="btn btn-primary" :disabled="busy || !target || !canClaim(target)" @click="claim">{{ busy ? '正在领取' : `确认领取 · ${label(duration)}` }}</button><button v-else-if="modal === 'release'" class="btn btn-primary" :disabled="busy" @click="release">确认{{ admin ? '回收' : '归还' }}</button><button v-else-if="modal === 'delete'" class="btn ex-delete-confirm" :disabled="busy" @click="removeChannel">{{ busy ? '正在删除' : '确认删除' }}</button><button v-else-if="modal === 'edit'" class="btn btn-primary" type="submit" form="exclusive-channel-form" :disabled="busy || !selectedAccounts.length">{{ busy ? '正在同步' : target ? '保存配置' : '发布通道' }}</button></div></template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<style scoped>
.ex-page,.ex-dialog { --ex-border:#e3e7ea; --ex-muted:#697582; color:#202a31; font-size:14px; }
.dark .ex-page,.dark .ex-dialog { --ex-border:#374348; --ex-muted:#a3b0b5; color:#e5ecef; }
.ex-heading,.ex-header-actions,.ex-card-top,.ex-toolbar,.ex-actions,.ex-occupied { display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap; }
.ex-heading { margin-bottom:24px; }.ex-heading h1 { font-size:25px; font-weight:650; }.ex-page h2 { font-size:19px; font-weight:600; }
.ex-eyebrow,.ex-muted,.ex-balance span { color:var(--ex-muted); font-size:12px; }.ex-balance { display:flex; flex-direction:column; gap:3px; }.ex-balance strong { color:#107e6c; font-size:16px; }
.ex-tabs { display:flex; gap:24px; border-bottom:1px solid var(--ex-border); }.ex-tabs button { padding:12px 0; border-bottom:2px solid transparent; }.ex-tabs button.selected { color:#107e6c; border-color:#107e6c; }
.ex-feedback { min-height:38px; padding:10px 0; font-size:13px; color:#107e6c; }.ex-error { color:#be343d; }.ex-warning { color:#a66a19; }
.ex-toolbar { margin:8px 0 20px; }.ex-toolbar input { margin-left:auto; max-width:230px; }.ex-page input:not([type='checkbox']),.ex-page select,.ex-dialog input:not([type='checkbox']),.ex-dialog select { border:1px solid var(--ex-border); border-radius:6px; background:transparent; color:inherit; padding:9px 11px; min-width:0; }
.ex-grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(min(100%,520px),620px)); align-items:start; justify-content:start; gap:16px; }.ex-channel { display:flex; flex-direction:column; align-self:start; background:white; padding:16px; border:1px solid var(--ex-border); border-radius:8px; min-width:0; }.ex-channel h2 { overflow-wrap:anywhere; margin:16px 0 5px; }.dark .ex-channel { background:#20292d; }
.ex-card-top { color:#107e6c; }.ex-status { font-size:12px; color:var(--ex-muted); }.ex-status[data-state='available'] { color:#107e6c; }.ex-status[data-state='active'] { color:#4c87c5; }.ex-status[data-state='cooling'],.ex-status[data-state='releasing'],.ex-status[data-state='error'] { color:#a66a19; }
.ex-countdown,.ex-account-summary,.ex-card-footer { display:flex; justify-content:space-between; gap:10px; font-size:12px; margin-top:14px; }.ex-countdown strong { font-variant-numeric:tabular-nums; }.ex-account-summary { color:var(--ex-muted); margin-top:18px; }.ex-card-footer { font-size:11px; color:var(--ex-muted); padding:2px 0 10px; }.ex-claim { width:100%; margin-top:0; }
.ex-occupied { padding:16px 0; border-bottom:1px solid var(--ex-border); margin-bottom:20px; }.ex-table-scroll { overflow-x:auto; }table { width:100%; min-width:650px; border-collapse:collapse; text-align:left; }th,td { padding:14px 12px; border-bottom:1px solid var(--ex-border); font-size:12px; }th { color:var(--ex-muted); font-weight:500; }td small { display:block; margin-top:4px; color:var(--ex-muted); }.ex-audit { margin-top:32px; }
.ex-pagination { display:flex; justify-content:center; align-items:center; gap:16px; margin-top:20px; }.ex-empty { display:flex; align-items:center; flex-direction:column; justify-content:center; gap:20px; min-height:220px; color:var(--ex-muted); }
.ex-lease-stats { display:flex; gap:32px; justify-content:space-between; padding:28px 0; flex-wrap:wrap; }.ex-lease-stats strong { display:block; margin:12px 0; font-size:26px; font-variant-numeric:tabular-nums; }.ex-lease-stats small { display:block; color:var(--ex-muted); }
.ex-section { border-top:1px solid var(--ex-border); padding:24px 0; }.ex-section h3 { margin-bottom:16px; font-size:16px; font-weight:600; }.ex-section-heading { display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap; margin-bottom:16px; }.ex-section-heading h3 { margin-bottom:0; }.ex-credential { display:flex; align-items:center; gap:12px; margin-top:12px; }.ex-credential>span { width:75px; flex-shrink:0; font-size:12px; color:var(--ex-muted); }.ex-credential input { flex:1; font-family:monospace; font-size:12px; }.ex-return { padding:20px 0; display:flex; justify-content:flex-end; }
.ex-dialog h3 { font-size:17px; font-weight:600; margin-bottom:20px; overflow-wrap:anywhere; }.ex-dialog p { line-height:1.8; }.ex-field { display:flex; flex-direction:column; gap:8px; margin-bottom:20px; font-size:13px; }.ex-facts { margin:16px 0; }.ex-facts>div { display:flex; justify-content:space-between; gap:14px; padding:12px 0; border-bottom:1px solid var(--ex-border); font-size:13px; }.ex-facts dt { color:var(--ex-muted); flex-shrink:0; }.ex-facts dd { text-align:right; overflow-wrap:anywhere; }
.ex-warning-box { display:flex; align-items:flex-start; gap:8px; padding:12px; margin:16px 0; border:1px solid #f0cd71; border-radius:6px; background:#fffbeb; color:#92620b; font-size:13px; line-height:1.8; }.ex-warning-box svg { flex-shrink:0; margin-top:4px; }.dark .ex-warning-box { background:#332b19; color:#f3cc72; border-color:#715824; }
.ex-delete { color:#be343d; }.ex-delete-confirm { background:#be343d; color:white; }.ex-delete-confirm:hover:not(:disabled) { background:#a72d35; }.dark .ex-delete { color:#f08a91; }
.ex-pricing-cell .ex-pending-rate,.ex-pricing-cell .ex-pending-note { color:#a66a19; }.ex-pricing-cell .ex-pending-note { font-weight:600; }.dark .ex-pricing-cell .ex-pending-rate,.dark .ex-pricing-cell .ex-pending-note { color:#e6b765; }
.ex-picker { border:1px solid var(--ex-border); border-radius:6px; }.ex-picker summary { cursor:pointer; display:flex; justify-content:space-between; padding:12px; }.ex-picker-body { padding:10px; border-top:1px solid var(--ex-border); max-height:280px; overflow-y:auto; }.ex-picker-body .ex-actions { flex-wrap:nowrap; }.ex-picker-body input[type='search'] { width:100%; }.ex-option { display:flex; align-items:center; gap:10px; padding:12px 0; }.ex-option span { overflow-wrap:anywhere; }.ex-option small { display:block; color:var(--ex-muted); }.ex-durations { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:14px 8px; padding-top:14px; }.ex-durations label { display:flex; gap:6px; align-items:center; font-size:12px; }input[type='checkbox'] { accent-color:#107e6c; }.ex-dialog-footer { justify-content:flex-end; }
@media(max-width:1250px){.ex-grid { grid-template-columns:repeat(auto-fit,minmax(min(100%,440px),1fr)); }}
@media(max-width:650px){.ex-grid { grid-template-columns:minmax(0,1fr); }.ex-heading h1 { font-size:22px; }.ex-toolbar input { max-width:none; width:100%; }.ex-lease-stats { gap:22px; }.ex-lease-stats>div:first-child { width:100%; }.ex-credential { flex-wrap:wrap; }.ex-credential>span { width:100%; }.ex-durations { grid-template-columns:repeat(2,minmax(0,1fr)); }.ex-dialog input:not([type='checkbox']),.ex-dialog select { font-size:16px; }}
.ex-workspace { display:grid; grid-template-columns:minmax(0,1fr); align-items:start; }.ex-workspace-main { min-width:0; }
.ex-channel { position:relative; border-top:3px solid #cbd5dc; transition:transform .18s ease,box-shadow .18s ease,border-color .18s ease; animation:channel-enter .32s ease backwards; animation-delay:var(--entry-delay,0ms); }
.ex-channel[data-state='available'] { border-top-color:#15947b; }.ex-channel[data-state='active'] { border-top-color:#5089d0; }.ex-channel[data-state='cooling'],.ex-channel[data-state='releasing'] { border-top-color:#d9a23c; }
.ex-card-top { align-items:flex-start; gap:16px; }.ex-card-identity { display:flex; align-items:center; gap:10px; padding-top:6px; }.ex-card-identity>svg { color:#718096; }
.ex-card-identity .ex-status { display:inline-flex; align-items:center; gap:6px; font-weight:600; }.ex-card-identity .ex-status::before { content:''; width:6px; height:6px; border-radius:50%; background:currentColor; }
.ex-rate { display:flex; flex-direction:column; align-items:flex-end; flex-shrink:0; gap:2px; max-width:55%; overflow-wrap:anywhere; }.ex-rate>span { color:var(--ex-muted); font-size:11px; }.ex-rate strong { color:#2563b4; font-size:30px; line-height:1.2; font-weight:700; font-variant-numeric:tabular-nums; }.ex-rate small { font-size:18px; margin-left:3px; font-weight:500; }
.ex-channel h2 { margin:8px 0 6px; font-size:20px; font-weight:650; line-height:1.5; }.ex-card-subtitle { font-size:12px; color:var(--ex-muted); }
.ex-channel .ex-account-summary { margin-top:16px; padding-top:12px; border-top:1px solid var(--ex-border); font-size:12px; }.ex-account-summary strong { color:#107e6c; font-size:14px; }
.ex-channel .ex-countdown { color:#536579; }.ex-channel .ex-countdown strong { color:#2563b4; font-size:14px; }.ex-channel .ex-idle-countdown,.ex-channel .ex-idle-countdown strong { color:#996515; }
.ex-channel :deep(.ex-account:last-child) { border-bottom:0; }.ex-channel :deep(.downgrade-test) { margin-top:2px; }.ex-card-footer { flex-wrap:wrap; line-height:1.6; margin-top:14px; }
.ex-channel .btn { transition:background-color .15s ease,transform .15s ease,box-shadow .15s ease; }.ex-channel .btn:not(:disabled):active { transform:scale(.98); }.ex-channel button:focus-visible { outline:2px solid #3980cf; outline-offset:3px; }.ex-claim { min-height:40px; gap:8px; }.ex-claim svg { transition:transform .18s ease; }
.ex-channel { background:linear-gradient(135deg,#edf9f5 0%,#ffffff 36%,#f0f5ff 100%); box-shadow:0 2px 3px #193b4710,0 6px 18px #193b4708,inset 0 1px 0 #ffffff; transform:perspective(1100px) rotateX(0deg) rotateY(0deg); }
.ex-channel[data-state='active'] { background:linear-gradient(135deg,#eaf2ff 0%,#ffffff 40%,#effaf6 100%); }.ex-channel[data-state='cooling'],.ex-channel[data-state='releasing'] { background:linear-gradient(135deg,#fff8e6 0%,#ffffff 40%,#f2f6fb 100%); }
.dark .ex-channel { background:linear-gradient(135deg,#233a35 0%,#20292d 40%,#273344 100%); box-shadow:0 4px 16px #00000030,inset 0 1px 0 #ffffff0d; }.dark .ex-channel[data-state='active'] { background:linear-gradient(135deg,#273e55 0%,#20292d 42%,#263c35 100%); }.dark .ex-channel[data-state='cooling'],.dark .ex-channel[data-state='releasing'] { background:linear-gradient(135deg,#3d3728 0%,#20292d 42%,#283542 100%); }
@media(hover:hover) and (pointer:fine){.ex-channel:hover { transform:perspective(1100px) translateY(-4px) rotateX(var(--tilt-x,0deg)) rotateY(var(--tilt-y,0deg)); box-shadow:0 14px 30px #193b471a,0 3px 6px #193b4712,inset 0 1px 0 #ffffff80; }.ex-claim:not(:disabled):hover svg { transform:translateX(3px); }}
@keyframes channel-enter { from { opacity:0; transform:translateY(10px); } to { opacity:1; transform:translateY(0); } }
.dark .ex-rate strong,.dark .ex-channel .ex-countdown strong { color:#85b6fa; }.dark .ex-account-summary strong,.dark .ex-status[data-state='available'] { color:#7acdbb; }.dark .ex-channel .ex-countdown { color:#adbdcf; }.dark .ex-channel .ex-idle-countdown,.dark .ex-channel .ex-idle-countdown strong { color:#e6b765; }
@media(prefers-reduced-motion:reduce){.ex-channel,.ex-channel .btn,.ex-claim svg { animation:none; transition:none; }.ex-channel:hover,.ex-channel .btn:not(:disabled):active,.ex-claim:not(:disabled):hover svg { transform:none; }}
</style>
