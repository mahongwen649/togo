<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import {
  advance, claim, countdown, createDemo, DEFAULT_DURATION, durationLabel, durationOptions,
  groupOf, idleDeadline, nextTransition, occupiedBy, phaseNames, reasonNames, release, saveChannel,
  selectableAccounts, setAvailability, simulateRequest, STORAGE_KEY,
  type Channel, type ChannelConfig, type DemoState,
} from './model'

function restore(): DemoState {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) {
      const saved = JSON.parse(raw) as DemoState
      if (saved.version === 1 && saved.channels.length && saved.users.length === 3 && Number.isFinite(saved.now)) {
        advance(saved, saved.now + Math.max(0, Date.now() - saved.realSavedAt))
        return saved
      }
    }
  } catch { /* A corrupt or unavailable demo cache starts a fresh preview. */ }
  return createDemo()
}
const state = reactive(restore())
const view = ref<'lobby' | 'mine' | 'history' | 'admin'>('lobby')
const userId = ref(1)
const dark = ref(false)
const search = ref('')
const filter = ref('all')
const notice = ref('')
const error = ref('')
const modal = ref<'claim' | 'release' | 'edit' | 'reset' | null>(null)
const targetId = ref<number | null>(null)
const chosenDuration = ref(DEFAULT_DURATION)
const form = reactive<ChannelConfig>({ accountIds: [], name: '', durations: [...durationOptions], cooldownSeconds: 60 })
const accountSearch = ref('')
const availableAccounts = computed(() => selectableAccounts(state))
const filteredAccounts = computed(() => availableAccounts.value.filter(a =>
  `${a.name} ${a.id}`.toLowerCase().includes(accountSearch.value.toLowerCase()),
))
const user = computed(() => state.users.find(u => u.id === userId.value)!)
const mine = computed(() => occupiedBy(state, userId.value))
const target = computed(() => state.channels.find(c => c.id === targetId.value))
const visibleChannels = computed(() => state.channels.filter(c => c.phase !== 'unpublished'))
const filteredChannels = computed(() => visibleChannels.value.filter(c =>
  (filter.value === 'all' || c.phase === filter.value) &&
  `${c.name} ${groupOf(state, c).models.join(' ')}`.toLowerCase().includes(search.value.toLowerCase()),
))
const availableCount = computed(() => visibleChannels.value.filter(c => c.phase === 'available').length)
const myHistory = computed(() => state.history.filter(h => h.userId === userId.value))
const titles = { lobby: '通道大厅', mine: '我的通道', history: '使用记录', admin: '独享通道管理' }
const money = (n: number) => `¥${n.toFixed(2)}`
const count = (n: number) => new Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 2 }).format(n)
const stamp = (n: number) => new Date(n).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
const clock = computed(() => new Date(state.now).toLocaleTimeString('zh-CN', { hour12: false }))
const modelGroup = (c: Channel) => groupOf(state, c)
const healthy = (c: Channel) => modelGroup(c).accounts.filter(a => a.available).length
const isMine = (c: Channel) => c.lease?.userId === userId.value
const status = (c: Channel) => isMine(c) && c.phase === 'active' ? '我在使用' : phaseNames[c.phase]
const canClaim = (c: Channel) => c.phase === 'available' && !mine.value && user.value.balance > 0 && healthy(c) > 0
function actionLabel(c: Channel) {
  if (isMine(c)) return '查看我的通道'
  if (c.phase === 'cooling') return `冷却 ${countdown(c.cooldownUntil!, state.now)}`
  if (c.phase !== 'available') return phaseNames[c.phase]
  if (mine.value) return '已有使用中的通道'
  if (user.value.balance <= 0) return '中转余额不足'
  return '领取通道'
}
function persist() {
  state.realSavedAt = Date.now()
  try { sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state)) } catch { /* The preview also works in memory. */ }
}
function run(action: () => void, message = '') {
  error.value = ''
  try { action(); persist(); notice.value = message } catch (e) { error.value = e instanceof Error ? e.message : '操作失败' }
}
function close() { modal.value = null; error.value = '' }
function open(kind: typeof modal.value, c?: Channel) {
  error.value = ''; targetId.value = c?.id ?? null; modal.value = kind; chosenDuration.value = DEFAULT_DURATION
}
function doClaim() {
  run(() => { claim(state, targetId.value!, userId.value, chosenDuration.value); close(); view.value = 'mine' }, '通道已领取')
}
function doRelease() {
  run(() => { release(state, targetId.value!, userId.value, view.value === 'admin' ? 'admin' : 'manual'); close() }, '已开始回收，等待清理完成')
}
function edit(c?: Channel) {
  accountSearch.value = ''
  Object.assign(form, { id: c?.id, groupId: c?.groupId, accountIds: c ? undefined : [], name: c?.name ?? '',
    durations: c ? [...c.durations] : [...durationOptions], cooldownSeconds: c?.cooldownSeconds ?? 60 })
  open('edit', c)
}
function save() { run(() => { saveChannel(state, form); close() }, '通道配置已保存') }
function changeAvailability(c: Channel, event: Event) {
  const select = event.target as HTMLSelectElement
  run(() => setAvailability(state, c.id, select.value as 'available' | 'maintenance' | 'unpublished'), '发布状态已更新')
  select.value = c.phase
}
function jump() {
  run(() => advance(state, nextTransition(state, userId.value)), '演示时钟已推进')
}
function request() { run(() => simulateRequest(state, userId.value), '已发起模拟请求，20 秒后完成') }
function reset() {
  Object.assign(state, createDemo()); userId.value = 1; view.value = 'lobby'; filter.value = 'all'; search.value = ''
  close(); persist(); notice.value = '演示数据已重置'
}
function switchUser() { close(); notice.value = ''; error.value = '' }
function toggleTheme() { dark.value = !dark.value; document.documentElement.classList.toggle('dark', dark.value) }
async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); notice.value = '已复制演示凭据' } catch { error.value = '无法访问剪贴板，请选择凭据文本复制' }
}
let timer: ReturnType<typeof setInterval>
let lastTick = Date.now()
onMounted(() => {
  document.documentElement.classList.remove('dark')
  timer = setInterval(() => {
    const now = Date.now(); advance(state, state.now + Math.max(0, now - lastTick)); lastTick = now; persist()
  }, 1000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="channel-demo">
    <div class="preview-bar">
      <span class="preview-label"><Icon name="beaker" size="sm" /> 前端预览 <span class="preview-separator">/</span> 仅假数据</span>
      <div class="preview-actions">
        <span class="demo-clock"><Icon name="clock" size="sm" /> {{ clock }}</span>
        <button class="demo-button compact" :disabled="mine?.phase !== 'active'" @click="request"><Icon name="play" size="sm" /> 模拟请求</button>
        <button class="demo-button compact" @click="jump"><Icon name="clock" size="sm" /> {{ mine?.phase === 'active' ? '跳到回收' : '下一阶段' }}</button>
        <button class="icon-button" aria-label="重置演示数据" title="重置演示数据" @click="open('reset')"><Icon name="refresh" size="sm" /></button>
      </div>
    </div>
    <div class="demo-shell">
      <aside class="demo-sidebar">
        <div class="brand"><img src="/logo.png" alt="" /><span>TogoAPI</span></div>
        <div class="nav-label">独享通道</div>
        <nav aria-label="独享通道导航">
          <button :class="{ selected: view === 'lobby' }" @click="view = 'lobby'"><Icon name="grid" /> 通道大厅 <span class="nav-count">{{ availableCount }}</span></button>
          <button :class="{ selected: view === 'mine' }" @click="view = 'mine'"><Icon name="server" /> 我的通道 <span v-if="mine" class="nav-dot"></span></button>
          <button :class="{ selected: view === 'history' }" @click="view = 'history'"><Icon name="chart" /> 使用记录</button>
        </nav>
        <div class="nav-label management-label">管理</div>
        <nav aria-label="管理员导航">
          <button :class="{ selected: view === 'admin' }" @click="view = 'admin'"><Icon name="cog" /> 通道管理</button>
        </nav>
        <div class="sidebar-bottom"><Icon name="badge" size="sm" /><span>独享资源 · 按量计费</span></div>
      </aside>
      <div class="workspace">
        <header class="workspace-header">
          <div class="breadcrumb">独享通道 <Icon name="chevronRight" size="xs" /><span>{{ titles[view] }}</span></div>
          <div class="header-actions">
            <div class="balance"><span>中转余额</span><strong>{{ money(user.balance) }}</strong></div>
            <button class="icon-button" :aria-label="dark ? '切换浅色' : '切换深色'" :title="dark ? '切换浅色' : '切换深色'" @click="toggleTheme"><Icon :name="dark ? 'sun' : 'moon'" /></button>
            <label class="identity"><span class="avatar">{{ user.name.slice(0, 1) }}</span><select v-model.number="userId" aria-label="演示用户" @change="switchUser"><option v-for="u in state.users" :key="u.id" :value="u.id">{{ u.name }}</option></select></label>
          </div>
        </header>
        <main>
          <div class="page-heading"><div><div class="eyebrow">EXCLUSIVE CHANNELS</div><h1>{{ titles[view] }}</h1></div><button v-if="view === 'admin'" class="demo-button primary" @click="edit()"><Icon name="plus" size="sm" /> 发布通道</button><span v-else class="mode-label"><Icon name="dollar" size="sm" /> 中转额度计费</span></div>
          <div class="feedback" aria-live="polite"><span v-if="error && !modal" role="alert" class="error-text">{{ error }}</span><span v-else>{{ notice }}</span></div>

          <template v-if="view === 'lobby'">
            <div class="overview-line"><span><strong>{{ availableCount }}</strong> 个空闲</span><span><strong>{{ visibleChannels.filter(c => c.phase === 'active' || c.phase === 'releasing').length }}</strong> 个使用中</span><span><strong>{{ visibleChannels.length }}</strong> 个通道</span><span class="overview-end">默认使用时长 <b>24 小时</b></span></div>
            <div v-if="mine" class="active-strip"><Icon name="server" /><div><strong>{{ mine.name }}</strong><span>{{ mine.phase === 'active' ? '剩余 ' + countdown(mine.lease!.expiresAt, state.now) : '正在归还，等待在途请求结束' }}</span></div><button class="demo-button" @click="view = 'mine'">查看通道 <Icon name="chevronRight" size="sm" /></button></div>
            <div class="lobby-toolbar"><div class="status-tabs" aria-label="通道状态"><button v-for="f in [{ id: 'all', label: '全部通道' }, { id: 'available', label: '空闲' }, { id: 'active', label: '使用中' }, { id: 'cooling', label: '冷却中' }]" :key="f.id" :aria-pressed="filter === f.id" :class="{ active: filter === f.id }" @click="filter = f.id">{{ f.label }}</button></div><label class="search-field"><Icon name="search" size="sm" /><input v-model="search" placeholder="搜索通道或模型" aria-label="搜索通道或模型" /></label></div>
            <div class="channel-grid">
              <article v-for="c in filteredChannels" :key="c.id" class="channel-card" :class="{ 'my-card': isMine(c) }" :data-channel="c.id">
                <div class="card-top"><span class="resource-icon"><Icon name="server" size="lg" /></span><span class="status" :data-phase="c.phase"><i></i>{{ status(c) }}</span></div>
                <div class="channel-title"><h2>{{ c.name }}</h2><span>{{ modelGroup(c).platform }} <span class="dot-separator">·</span> 专属标准分组</span></div>
                <div class="model-labels"><span v-for="model in modelGroup(c).models" :key="model">{{ model }}</span></div>
                <div v-if="c.phase === 'active' && c.lease" class="release-countdown"><span><Icon name="clock" size="sm" /> 到期回收倒计时</span><strong>{{ countdown(c.lease.expiresAt, state.now) }}</strong></div>
                <div v-else-if="c.phase === 'releasing'" class="release-countdown draining"><span>回收清理中</span><strong>暂不可领取</strong></div>
                <div v-if="c.phase === 'active' && c.lease" class="release-countdown idle-countdown"><span>无消费回收倒计时</span><strong>{{ countdown(idleDeadline(c.lease), state.now) }}</strong></div>
                <div class="account-line"><span>可用账号 <strong>{{ healthy(c) }} <small>/ {{ modelGroup(c).accounts.length }}</small></strong></span><span>剩余额度</span></div>
                <div class="inline-accounts">
                  <div v-for="a in modelGroup(c).accounts" :key="a.name" class="inline-account">
                    <div class="inline-account-heading"><strong>{{ a.name }}</strong><span :class="a.available ? 'healthy-text' : 'warning-text'">{{ a.available ? '可用' : '额度受限' }}</span></div>
                    <div class="quota-grid">
                      <div><div class="quota-label"><span>5 小时剩余</span><b>{{ 100 - a.fiveHour }}%</b></div><progress :value="100 - a.fiveHour" max="100" :aria-label="a.name + ' 5 小时剩余'" :class="{ limited: !a.available }"></progress></div>
                      <div><div class="quota-label"><span>7 天剩余</span><b>{{ 100 - a.weekly }}%</b></div><progress :value="100 - a.weekly" max="100" :aria-label="a.name + ' 7 天剩余'" class="weekly"></progress></div>
                    </div>
                  </div>
                </div>
                <dl class="card-metrics"><div><dt>请求数</dt><dd>{{ count(c.todayRequests) }}</dd></div><div><dt>Token 用量</dt><dd>{{ count(c.todayTokens) }}</dd></div><div><dt>消费金额</dt><dd>{{ money(c.todayCost) }}</dd></div></dl>
                <div class="rate-line"><span>计费倍率 <b>{{ modelGroup(c).multiplier.toFixed(1) }}×</b></span><span>归还后冷却 {{ c.cooldownSeconds }} 秒</span></div>
                <button class="demo-button full" :class="{ primary: canClaim(c), secondary: isMine(c) }" :disabled="!canClaim(c) && !isMine(c)" @click="isMine(c) ? view = 'mine' : open('claim', c)"><Icon v-if="canClaim(c)" name="login" size="sm" />{{ actionLabel(c) }}</button>
                <div class="card-footnote">{{ c.phase === 'active' ? '到期后需完成清理和冷却，才可再次领取' : c.phase === 'releasing' ? '等待在途请求结束，清理完成后进入冷却' : c.phase === 'maintenance' ? '管理员维护中，暂不开放' : '数据快照 ' + stamp(c.updatedAt) }}</div>
              </article>
            </div>
            <div v-if="!filteredChannels.length" class="empty"><Icon name="search" size="xl" /><h2>没有匹配的通道</h2><button class="demo-button" @click="search = ''; filter = 'all'">查看全部通道</button></div>
          </template>
          <template v-else-if="view === 'mine'">
            <section v-if="mine?.lease" class="my-channel" data-test="my-channel">
              <div class="my-heading"><span class="resource-icon"><Icon name="server" size="lg" /></span><div><h2>{{ mine.name }}</h2><span>{{ modelGroup(mine).platform }} · {{ healthy(mine) }} 个可用账号 · {{ modelGroup(mine).multiplier.toFixed(1) }}×</span></div><span class="status" :data-phase="mine.phase"><i></i>{{ status(mine) }}</span></div>
              <div class="lease-overview"><div><span class="field-label">{{ mine.phase === 'active' ? '剩余使用时间' : '回收状态' }}</span><strong class="lease-timer">{{ mine.phase === 'active' ? countdown(mine.lease.expiresAt, state.now) : '等待清理完成' }}</strong><span class="secondary-text">{{ mine.phase === 'active' ? stamp(mine.lease.expiresAt) + ' 到期' : '在途请求 ' + mine.lease.requests.length + ' 个' }}</span><div v-if="mine.phase === 'active'" class="release-countdown idle-countdown"><span>无消费回收倒计时</span><strong>{{ countdown(idleDeadline(mine.lease), state.now) }}</strong></div><div v-else-if="mine.lease.reason" class="secondary-text">{{ reasonNames[mine.lease.reason] }}</div></div><dl class="lease-stats"><div><dt>本次消费</dt><dd>{{ money(mine.lease.cost) }}</dd></div><div><dt>完成请求</dt><dd>{{ mine.lease.completed }}</dd></div><div><dt>本次 Token</dt><dd>{{ count(mine.lease.tokens) }}</dd></div></dl></div>
              <ol class="lifecycle"><li class="done"><Icon name="check" size="sm" /> 领取成功</li><li :class="{ current: mine.phase === 'active', done: mine.phase === 'releasing' }">使用中</li><li :class="{ current: mine.phase === 'releasing' }">归还清理</li><li>冷却 {{ mine.cooldownSeconds }} 秒</li></ol>
              <section class="credentials"><div class="section-heading"><h3>接入信息</h3><span class="subtle-label">演示凭据，无实际调用权限</span></div><label class="credential-row"><span>Base URL</span><input readonly value="https://demo.invalid/v1" aria-label="演示 Base URL" /><button class="icon-button" title="复制演示 Base URL" aria-label="复制演示 Base URL" @click="copy('https://demo.invalid/v1')"><Icon name="copy" size="sm" /></button></label><label class="credential-row"><span>API Key</span><input readonly :value="mine.lease.key" aria-label="演示 API Key" /><button class="icon-button" :disabled="mine.phase !== 'active'" title="复制演示 Key" aria-label="复制演示 Key" @click="copy(mine.lease.key)"><Icon name="copy" size="sm" /></button></label></section>
              <section v-if="mine.lease.requests.length" class="inflight"><div class="section-heading"><h3>在途请求</h3><span class="subtle-label">{{ mine.lease.requests.length }} / 3 并发</span></div><div v-for="r in mine.lease.requests" :key="r.id" class="request-row"><span class="pulse-dot"></span><span>模拟请求 #{{ r.id }}</span><span class="secondary-text">剩余 {{ countdown(r.endsAt, state.now) }}</span><span>{{ money(r.cost) }}</span></div></section>
              <section class="my-account-usage" aria-labelledby="my-account-usage-title">
                <div class="section-heading"><h3 id="my-account-usage-title">账号用量</h3><span class="subtle-label">可用账号 {{ healthy(mine) }} / {{ modelGroup(mine).accounts.length }}</span></div>
                <div class="account-detail-list">
                  <div v-for="a in modelGroup(mine).accounts" :key="a.name" class="account-detail">
                    <div class="section-heading"><strong>{{ a.name }}</strong><span :class="a.available ? 'healthy-text' : 'warning-text'">{{ a.available ? '可用' : '额度受限' }}</span></div>
                    <div class="quota-grid">
                      <div><div class="quota-label"><span>5 小时剩余</span><b>{{ 100 - a.fiveHour }}%</b></div><progress :value="100 - a.fiveHour" max="100" :aria-label="a.name + ' 5 小时剩余'" :class="{ limited: !a.available }"></progress></div>
                      <div><div class="quota-label"><span>7 天剩余</span><b>{{ 100 - a.weekly }}%</b></div><progress :value="100 - a.weekly" max="100" :aria-label="a.name + ' 7 天剩余'" class="weekly"></progress></div>
                    </div>
                  </div>
                </div>
                <div class="secondary-text">数据快照 {{ stamp(mine.updatedAt) }}</div>
              </section>
              <div class="my-actions"><button class="demo-button danger" :disabled="mine.phase !== 'active'" @click="open('release', mine)"><Icon name="swap" size="sm" />{{ mine.phase === 'active' ? '归还通道' : '归还处理中' }}</button></div>
            </section>
            <div v-else class="empty"><span class="empty-icon"><Icon name="server" size="xl" /></span><h2>暂无正在使用的通道</h2><button class="demo-button primary" @click="view = 'lobby'">前往通道大厅 <Icon name="chevronRight" size="sm" /></button><button v-if="myHistory.length" class="text-button" @click="view = 'history'">查看使用记录</button></div>
          </template>
          <template v-else-if="view === 'history'">
            <div class="section-heading"><h2>我的使用记录</h2><span class="secondary-text">{{ myHistory.length }} 次使用</span></div>
            <div class="demo-table-wrap"><table class="demo-table"><thead><tr><th>通道</th><th>领取时间</th><th>结束时间</th><th>结束方式</th><th>请求 / Token</th><th class="align-right">消费</th></tr></thead><tbody><tr v-for="h in myHistory" :key="h.id"><td><strong>{{ h.channelName }}</strong><span class="cell-note">记录 #{{ h.id }}</span></td><td>{{ stamp(h.startedAt) }}</td><td>{{ stamp(h.endedAt) }}</td><td>{{ reasonNames[h.reason] }}</td><td>{{ h.requests }} / {{ count(h.tokens) }}</td><td class="align-right">{{ money(h.cost) }}</td></tr></tbody></table></div><div v-if="!myHistory.length" class="empty"><Icon name="chart" size="xl" /><h2>暂无使用记录</h2></div>
          </template>
          <template v-else>
            <div class="overview-line"><span>已配置 <strong>{{ state.channels.length }}</strong> 个通道</span><span>可用 <strong>{{ availableCount }}</strong></span><span class="overview-end">管理员视角</span></div>
            <div class="demo-table-wrap"><table class="demo-table admin-table"><thead><tr><th>通道 / 分组</th><th>状态</th><th>当前使用者</th><th>冷却时间</th><th>可选时长</th><th class="align-right">操作</th></tr></thead><tbody><tr v-for="c in state.channels" :key="c.id" :data-admin-channel="c.id"><td><strong>{{ c.name }}</strong><span class="cell-note">{{ modelGroup(c).name }} · #{{ c.groupId }}</span></td><td><span class="status" :data-phase="c.phase"><i></i>{{ phaseNames[c.phase] }}</span><span v-if="c.cooldownUntil" class="cell-note tabular">{{ countdown(c.cooldownUntil, state.now) }}</span></td><td><template v-if="c.lease">{{ state.users.find(u => u.id === c.lease?.userId)?.name }}<span class="cell-note">{{ c.phase === 'releasing' ? '等待在途请求结束' : stamp(c.lease.expiresAt) + ' 到期' }}</span></template><span v-else class="secondary-text">未占用</span></td><td>{{ c.cooldownSeconds }} 秒</td><td><span class="cell-note duration-cell">{{ c.durations.map(durationLabel).join(' / ') }}</span></td><td><div class="table-actions"><button class="icon-button" :aria-label="'编辑 ' + c.name" :title="'编辑 ' + c.name" @click="edit(c)"><Icon name="edit" size="sm" /></button><button v-if="c.phase === 'active'" class="demo-button compact danger" @click="open('release', c)">回收</button><select v-else :value="c.phase" :disabled="c.phase === 'releasing' || c.phase === 'cooling'" :aria-label="c.name + ' 发布状态'" @change="changeAvailability(c, $event)"><option v-if="c.phase === 'releasing' || c.phase === 'cooling'" :value="c.phase">{{ phaseNames[c.phase] }}</option><option value="available">开放</option><option value="maintenance">维护</option><option value="unpublished">下架</option></select></div></td></tr></tbody></table></div>
            <section class="event-section"><div class="section-heading"><h2>操作记录</h2><span class="secondary-text">最近 {{ state.events.length }} 条</span></div><div class="demo-table-wrap"><table class="demo-table"><thead><tr><th>时间</th><th>通道</th><th>操作</th><th>操作者</th></tr></thead><tbody><tr v-for="e in state.events.slice(0, 20)" :key="e.id"><td>{{ stamp(e.at) }}</td><td>{{ e.channelName }}</td><td>{{ e.action }}</td><td>{{ e.actor }}</td></tr></tbody></table></div></section>
          </template>
          <footer class="workspace-footer"><span>TogoAPI Portal</span><span>独享通道 / 交互预览</span></footer>
        </main>
      </div>
    </div>

    <BaseDialog :show="modal !== null" :title="modal === 'claim' ? '领取通道' : modal === 'release' ? (view === 'admin' ? '强制回收通道' : '归还通道') : modal === 'edit' ? (form.id ? '编辑通道' : '发布通道') : '重置演示数据'" width="normal" @close="close">
      <div class="demo-modal-content">
        <div v-if="error" role="alert" class="form-error">{{ error }}</div>
        <template v-if="modal === 'claim' && target">
          <div class="claim-summary"><span class="resource-icon"><Icon name="server" size="lg" /></span><div><h3>{{ target.name }}</h3><span>{{ healthy(target) }} 个可用账号 · {{ modelGroup(target).multiplier.toFixed(1) }}× 倍率</span></div></div>
          <label class="form-field"><span>使用时长</span><select v-model.number="chosenDuration" aria-label="使用时长"><option v-for="d in target.durations" :key="d" :value="d">{{ durationLabel(d) }}{{ d === DEFAULT_DURATION ? '（默认）' : '' }}</option></select></label>
          <dl class="confirmation-facts"><div><dt>预计到期</dt><dd>{{ stamp(state.now + chosenDuration * 60000) }}</dd></div><div><dt>归还后冷却</dt><dd>{{ target.cooldownSeconds }} 秒</dd></div><div><dt>扣费账户</dt><dd>中转余额 · {{ money(user.balance) }}</dd></div><div><dt>领取费用</dt><dd>¥0.00</dd></div></dl>
          <div class="idle-policy-warning"><Icon name="exclamationTriangle" size="sm" /><p><strong>连续 30 分钟未产生消费将自动回收。</strong>产生消费后重新计时；回收仍需等待在途请求结束并完成清理。</p></div>
        </template>
        <template v-else-if="modal === 'release' && target?.lease">
          <h3>{{ target.name }}</h3><dl class="confirmation-facts"><div><dt>本次消费</dt><dd>{{ money(target.lease.cost) }}</dd></div><div><dt>在途请求</dt><dd>{{ target.lease.requests.length }} 个</dd></div><div><dt>清理后冷却</dt><dd>{{ target.cooldownSeconds }} 秒</dd></div></dl><p class="release-notice">归还后停止接收新请求。已开始的请求结束、清理完成后，通道进入冷却。</p>
        </template>
        <form v-else-if="modal === 'edit'" id="demo-channel-form" class="admin-form" @submit.prevent="save">
          <label class="form-field"><span>通道名称</span><input v-model="form.name" required maxlength="40" aria-label="通道名称" placeholder="例如：开发通道 06" /></label>
          <div v-if="!form.id" class="form-field">
            <span id="channel-accounts-label">绑定账号 <span class="secondary-text">{{ availableAccounts.length }} 个可选</span></span>
            <details class="account-picker">
              <summary aria-labelledby="channel-accounts-label"><span>{{ form.accountIds?.length ? '已选择 ' + form.accountIds.length + ' 个账号' : '请选择可用账号' }}</span><Icon name="chevronDown" size="sm" /></summary>
              <div class="account-picker-options">
                <input v-model="accountSearch" type="search" aria-label="搜索可用账号" placeholder="搜索账号名称或 ID" />
                <div class="account-picker-list" role="group" aria-labelledby="channel-accounts-label">
                  <label v-for="a in filteredAccounts" :key="a.id" class="account-picker-option"><input v-model="form.accountIds" type="checkbox" :value="a.id" :aria-label="a.name" /><span><strong>{{ a.name }}</strong><small>#{{ a.id }} · 5 小时剩余 {{ 100 - a.fiveHour }}% · 7 天剩余 {{ 100 - a.weekly }}%</small></span></label>
                  <p v-if="!filteredAccounts.length" class="secondary-text">暂无匹配的可用账号</p>
                </div>
              </div>
            </details>
            <div v-if="form.accountIds?.length" class="selected-account-names">{{ availableAccounts.filter(a => form.accountIds?.includes(a.id)).map(a => a.name).join('、') }}</div>
          </div>
          <dl v-else-if="target" class="confirmation-facts"><div><dt>Core 分组</dt><dd>{{ modelGroup(target).name }} · #{{ target.groupId }}</dd></div><div><dt>已绑定账号</dt><dd>{{ modelGroup(target).accounts.map(a => a.name).join('、') }}</dd></div></dl>
          <label class="form-field"><span>归还后冷却时间（秒）</span><input v-model.number="form.cooldownSeconds" type="number" min="0" max="86400" step="1" required aria-label="归还后冷却时间（秒）" /></label>
          <fieldset><legend>可选使用时长</legend><div class="duration-options"><label v-for="d in durationOptions" :key="d"><input v-model="form.durations" type="checkbox" :value="d" :disabled="d === DEFAULT_DURATION" />{{ durationLabel(d) }}{{ d === DEFAULT_DURATION ? '（默认）' : '' }}</label></div></fieldset>
        </form>
        <p v-else-if="modal === 'reset'">清空本页的演示操作，恢复初始通道、用户和余额？真实业务数据不会受到影响。</p>
      </div>
      <template #footer><div class="modal-actions"><button class="demo-button" @click="close">取消</button><button v-if="modal === 'claim'" class="demo-button primary" :disabled="!target || !canClaim(target)" @click="doClaim">确认领取 · {{ durationLabel(chosenDuration) }}</button><button v-if="modal === 'release'" class="demo-button danger" @click="doRelease">确认{{ view === 'admin' ? '回收' : '归还' }}</button><button v-if="modal === 'edit'" class="demo-button primary" type="submit" form="demo-channel-form">{{ form.id ? '保存配置' : '发布通道' }}</button><button v-if="modal === 'reset'" class="demo-button danger" @click="reset">重置演示</button></div></template>
    </BaseDialog>
  </div>
</template>
