<template>
  <div v-if="!loggedIn" class="login-page">
    <div class="login-panel">
      <div class="login-brand brand"><img :src="logoUrl" alt="TogoAPI" /><span class="brand-copy"><strong>TogoAPI</strong><span>活动管理</span></span></div>
      <div class="login-heading"><span><ShieldCheck :size="23" /></span><div><h1>管理后台</h1><p>使用活动管理员密码登录</p></div></div>
      <form @submit.prevent="login">
        <div class="field"><label for="admin-email">管理员邮箱</label><div class="password-field"><Mail :size="17" /><input id="admin-email" v-model="adminEmail" class="input" type="email" placeholder="admin@example.com" required /></div></div>
        <div class="field"><label for="password">管理员密码</label><div class="password-field"><LockKeyhole :size="17" /><input id="password" v-model="password" class="input" type="password" placeholder="请输入管理员密码" required /></div></div>
        <button class="btn btn-primary" type="submit" :disabled="loggingIn"><RefreshCw v-if="loggingIn" class="spin" :size="17" /><ArrowRight v-else :size="17" />{{ loggingIn ? '正在登录' : '登录后台' }}</button>
      </form>
      <p class="login-foot">会话将在 12 小时后自动失效</p>
    </div>
  </div>

  <div v-else class="admin-app">
    <aside class="admin-sidebar">
      <div class="sidebar-brand brand"><img :src="logoUrl" alt="TogoAPI" /><span class="brand-copy"><strong>TogoAPI</strong><span>活动管理</span></span></div>
      <nav aria-label="管理导航">
        <button :class="{ active: page === 'campaigns' }" type="button" @click="page = 'campaigns'"><CalendarRange :size="18" /><span>活动管理</span></button>
        <button :class="{ active: page === 'payouts' }" type="button" @click="page = 'payouts'"><ReceiptText :size="18" /><span>发放记录</span><em v-if="failedCount">{{ failedCount }}</em></button>
      </nav>
      <div class="sidebar-bottom">
        <a href="/user-web/" target="_blank"><ExternalLink :size="16" />打开用户端</a>
        <button type="button" @click="logout"><LogOut :size="16" />退出登录</button>
      </div>
    </aside>

    <div class="admin-main">
      <header class="admin-header">
        <div><p>TogoAPI / 活动运营</p><h1>{{ page === 'campaigns' ? '活动管理' : '发放记录' }}</h1></div>
        <div class="admin-account"><span>AD</span><div><strong>活动管理员</strong><small>独立管理会话</small></div></div>
      </header>

      <main class="admin-content">
        <div v-if="dataError" class="service-state error-state"><CircleAlert :size="18" /><span><strong>数据加载失败</strong>{{ dataError }}</span><button class="btn btn-secondary btn-sm" type="button" @click="loadRealData"><RefreshCw :size="14" />重新加载</button></div>
        <template v-if="page === 'campaigns'">
          <div class="page-toolbar">
            <div><h2>全部活动</h2><p>管理活动配置、时间与开奖状态</p></div>
            <button class="btn btn-primary" type="button" @click="openCreate"><Plus :size="18" />创建活动</button>
          </div>

          <section class="stats-grid" aria-label="活动统计">
            <article><span class="stat-symbol teal"><UsersRound :size="20" /></span><div><p>本期报名</p><strong>{{ activeCampaign?.participants ?? 0 }}</strong><small>{{ activeCampaign ? `人数上限 ${activeCampaign.participantLimit}` : '暂无进行中的活动' }}</small></div></article>
            <article><span class="stat-symbol amber"><WalletCards :size="20" /></span><div><p>预计随机预算</p><strong>{{ formatAmount(activeEstimatedBudget) }}</strong><small>按当前随机奖人数比例缩减</small></div></article>
            <article><span class="stat-symbol blue"><CalendarClock :size="20" /></span><div><p>距离开奖</p><strong>{{ drawDistance }}</strong><small>{{ activeCampaign ? '到点自动封盘并开奖' : '等待新活动发布' }}</small></div></article>
            <article><span class="stat-symbol rose"><CircleAlert :size="20" /></span><div><p>待处理异常</p><strong>{{ failedCount }}</strong><small>来自往期发放</small></div></article>
          </section>

          <section v-if="activeCampaign" class="active-notice">
            <div class="notice-mark"><Radio :size="20" /></div>
            <div><span>当前活动</span><strong>{{ activeCampaign.name }}</strong><p>{{ statusText(activeCampaign.status) }}，服务将按配置自动执行</p></div>
            <div class="notice-progress"><span><b>{{ activeCampaign.participants }}</b> / {{ activeCampaign.participantLimit }} 人已报名</span><div><i :style="{ width: activeProgress + '%' }"></i></div></div>
            <button class="btn btn-secondary btn-sm" type="button" @click="openEdit(activeCampaign)"><Eye :size="15" />查看配置</button>
          </section>
          <section v-else-if="!dataLoading" class="service-state empty-state"><CalendarRange :size="20" /><span><strong>暂无进行中的活动</strong>创建并发布活动后，报名数据会在这里实时显示。</span></section>

          <section class="table-panel">
            <div class="table-tools">
              <div class="search-box"><Search :size="16" /><input v-model="campaignSearch" type="search" placeholder="搜索活动名称" /></div>
              <select v-model="campaignStatus" class="select compact-select"><option value="all">全部状态</option><option value="registering">报名中</option><option value="completed">已开奖</option><option value="draft">草稿</option><option value="cancelled">已取消</option></select>
            </div>
            <div class="table-scroll">
              <table>
                <thead><tr><th>活动</th><th>活动时间</th><th>报名 / 名额</th><th>随机预算</th><th>状态</th><th class="align-right">操作</th></tr></thead>
                <tbody>
                  <tr v-if="dataLoading"><td colspan="6"><div class="table-state"><RefreshCw class="spin" :size="17" />正在从服务加载活动</div></td></tr>
                  <tr v-for="campaign in filteredCampaigns" :key="campaign.id">
                    <td><div class="activity-cell"><span>{{ String(campaign.id).slice(-2) }}</span><div><strong>{{ campaign.name }}</strong><small>#{{ campaign.id }}</small></div></div></td>
                    <td><div class="date-cell"><strong>{{ formatFullDate(campaign.registrationStart) }}</strong><small>至 {{ formatFullDate(campaign.drawAt) }}</small></div></td>
                    <td><div class="quota-cell"><span><b>{{ campaign.participants }}</b> / {{ campaign.randomLimit }}</span><div><i :style="{ width: Math.min(100, campaign.participants / campaign.randomLimit * 100) + '%' }"></i></div></div></td>
                    <td><strong class="money-cell">{{ formatAmount(campaign.randomBudget) }}</strong><small class="amount-range">{{ formatAmount(campaign.randomMin) }}～{{ formatAmount(campaign.randomMax) }}</small></td>
                    <td><span class="badge" :class="statusClass(campaign.status)">{{ statusText(campaign.status) }}</span></td>
                    <td class="align-right"><div class="row-actions"><button class="icon-action" type="button" title="编辑配置" @click="openEdit(campaign)"><Pencil :size="16" /></button><button v-if="campaign.status === 'registering' || campaign.status === 'scheduled' || campaign.status === 'drawing'" class="icon-action" type="button" title="立即开奖" @click="confirmDraw(campaign)"><ChartNoAxesColumnIncreasing :size="16" /></button><button v-if="campaign.status === 'registering' || campaign.status === 'scheduled'" class="icon-action danger" type="button" title="取消活动" @click="confirmCancel(campaign)"><Ban :size="16" /></button><button v-if="campaign.status !== 'registering' && campaign.status !== 'scheduled' && campaign.status !== 'drawing'" class="icon-action" type="button" title="复制活动" @click="duplicateCampaign(campaign)"><Copy :size="16" /></button></div></td>
                  </tr>
                  <tr v-if="!dataLoading && filteredCampaigns.length === 0"><td colspan="6"><div class="table-state">{{ dataError ? '活动数据暂不可用' : '没有符合条件的活动' }}</div></td></tr>
                </tbody>
              </table>
            </div>
            <div class="table-footer"><span>共 {{ filteredCampaigns.length }} 期活动</span><div><button disabled><ChevronLeft :size="15" /></button><button class="current">1</button><button disabled><ChevronRight :size="15" /></button></div></div>
          </section>
        </template>

        <template v-else>
          <div class="page-toolbar"><div><h2>额度发放流水</h2><p>查询中奖结果、到账状态与异常记录</p></div><button class="btn btn-secondary" type="button" @click="exportRecords"><Download :size="17" />导出记录</button></div>
          <section class="payout-summary">
            <div><span>累计发放</span><strong>{{ formatAmount(creditedAmount) }}</strong><small>随机奖 {{ formatAmount(randomCreditedAmount) }} + 保底 {{ formatAmount(guaranteeCreditedAmount) }}</small></div>
            <div><span>成功到账</span><strong class="success-text">{{ creditedCount }}</strong><small>{{ payoutSuccessRate }}% 发放成功</small></div>
            <div><span>处理中</span><strong class="blue-text">{{ processingCount }}</strong><small>等待 Core 返回</small></div>
            <div><span>发放失败</span><strong class="danger-text">{{ failedCount }}</strong><small>可执行幂等重试</small></div>
          </section>
          <section class="table-panel">
            <div class="table-tools payout-tools">
              <div class="search-box"><Search :size="16" /><input v-model="recordSearch" type="search" placeholder="搜索脱敏邮箱" /></div>
              <select v-model="recordType" class="select compact-select"><option value="all">全部奖项</option><option value="random">随机奖</option><option value="guarantee">保底奖</option></select>
              <select v-model="recordStatus" class="select compact-select"><option value="all">全部状态</option><option value="pending">待处理</option><option value="credited">已到账</option><option value="processing">处理中</option><option value="failed">失败</option></select>
            </div>
            <div class="table-scroll">
              <table>
                <thead><tr><th>参与账户</th><th>所属活动</th><th>内部奖项</th><th>中奖额度</th><th>发放时间</th><th>状态</th><th class="align-right">操作</th></tr></thead>
                <tbody>
                  <tr v-if="dataLoading"><td colspan="7"><div class="table-state"><RefreshCw class="spin" :size="17" />正在从服务加载发放记录</div></td></tr>
                  <tr v-for="record in filteredRecords" :key="record.id">
                    <td><div class="email-cell"><span>{{ record.email.slice(0, 1).toUpperCase() }}</span><strong>{{ record.email }}</strong></div></td>
                    <td>{{ record.campaign }}</td><td><span class="internal-prize" :class="record.prizeType">{{ record.prizeType === 'random' ? '随机奖' : '保底奖' }}</span></td><td><strong class="money-cell">+{{ formatAmount(record.amount) }}</strong></td><td class="muted-cell">{{ formatFullDate(record.createdAt) }}</td><td><span class="badge" :class="payoutClass(record.status)">{{ payoutText(record.status) }}</span></td>
                    <td class="align-right"><button v-if="record.status === 'failed'" class="btn btn-secondary btn-sm" type="button" :disabled="retryingId === record.id" @click="retryRecord(record)"><RefreshCw :class="{ spin: retryingId === record.id }" :size="14" />{{ retryingId === record.id ? '重试中' : '重试' }}</button><span v-else class="muted-cell">--</span></td>
                  </tr>
                  <tr v-if="!dataLoading && filteredRecords.length === 0"><td colspan="7"><div class="table-state">{{ dataError ? '发放数据暂不可用' : '没有符合条件的发放记录' }}</div></td></tr>
                </tbody>
              </table>
            </div>
            <div class="table-footer"><span>当前显示 {{ filteredRecords.length }} 条记录</span><div><button disabled><ChevronLeft :size="15" /></button><button class="current">1</button><button><ChevronRight :size="15" /></button></div></div>
          </section>
        </template>
      </main>
    </div>

    <div v-if="drawerOpen" class="drawer-overlay" @click.self="closeDrawer">
      <aside class="config-drawer" role="dialog" aria-modal="true" aria-labelledby="drawer-title">
        <header><div><p>{{ editingId ? `活动 #${editingId}` : 'NEW CAMPAIGN' }}</p><h2 id="drawer-title">{{ editingId ? '活动配置' : '创建活动' }}</h2></div><button class="btn btn-ghost btn-icon" type="button" aria-label="关闭" @click="closeDrawer"><X :size="20" /></button></header>
        <div v-if="formLocked" class="lock-notice"><LockKeyhole :size="16" /><span>报名已开始，时间及额度配置不可修改。</span></div>
        <form @submit.prevent="saveCampaign">
          <div class="drawer-body">
            <section><h3>基本信息</h3><div class="form-grid"><div class="field full"><label for="campaign-name">活动名称</label><input id="campaign-name" v-model="form.name" class="input" required /></div><div class="field full"><label for="campaign-subtitle">活动副标题</label><input id="campaign-subtitle" v-model="form.subtitle" class="input" required /></div></div></section>
            <section><h3>活动时间</h3><div class="form-grid"><div class="field"><label for="registration-start">报名开始时间</label><input id="registration-start" v-model="form.registrationStart" class="input" type="datetime-local" :disabled="formLocked" required /></div><div class="field"><label for="draw-at">自动开奖时间</label><input id="draw-at" v-model="form.drawAt" class="input" type="datetime-local" :disabled="formLocked" required /></div></div><p class="field-hint">开奖时间同时作为报名截止时间，使用北京时间。</p></section>
            <section><h3>额度规则</h3><div class="form-grid thirds"><div class="field"><label for="participant-limit">报名人数上限</label><input id="participant-limit" v-model.number="form.participantLimit" class="input" type="number" min="1" :disabled="formLocked" required /></div><div class="field"><label for="random-limit">随机奖名额</label><input id="random-limit" v-model.number="form.randomLimit" class="input" type="number" min="1" :disabled="formLocked" required /></div><div class="field"><label for="random-min">最小额度</label><input id="random-min" v-model.number="form.randomMin" class="input" type="number" min="0.01" step="0.01" :disabled="formLocked" required /></div><div class="field"><label for="random-max">最大额度</label><input id="random-max" v-model.number="form.randomMax" class="input" type="number" min="0.01" step="0.01" :disabled="formLocked" required /></div><div class="field"><label for="random-budget">随机奖总预算</label><input id="random-budget" v-model.number="form.randomBudget" class="input" type="number" min="0.01" step="0.01" :disabled="formLocked" required /></div><div class="field"><label for="guarantee">固定保底额度</label><input id="guarantee" v-model.number="form.guaranteeAmount" class="input" type="number" min="0.01" step="0.01" :disabled="formLocked" required /></div></div>
              <div class="budget-check" :class="{ invalid: !budgetValid }"><div><Calculator :size="18" /><span>配置测算</span></div><dl><div><dt>人均预算</dt><dd>{{ averageBudget }}</dd></div><div><dt>当前预计发放</dt><dd>{{ currentScaledBudget }}</dd></div><div><dt>区间校验</dt><dd>{{ budgetValid ? '配置有效' : '超出随机区间' }}</dd></div></dl></div>
            </section>
            <section><h3>发布设置</h3><label class="toggle-row"><span><strong>保存后立即发布</strong><small>同一时间只能有一期已发布活动</small></span><input v-model="form.published" type="checkbox" :disabled="formLocked" /><i></i></label></section>
          </div>
          <footer><button class="btn btn-secondary" type="button" :disabled="saving" @click="closeDrawer">取消</button><button class="btn btn-primary" type="submit" :disabled="!budgetValid || saving"><RefreshCw v-if="saving" class="spin" :size="17" /><Save v-else :size="17" />{{ saving ? '保存中' : '保存活动' }}</button></footer>
        </form>
      </aside>
    </div>

    <div v-if="drawTarget" class="overlay" @click.self="drawTarget = null"><div class="dialog confirm-dialog"><div class="dialog-body"><span class="danger-icon"><ChartNoAxesColumnIncreasing :size="23" /></span><h2>立即开奖？</h2><p>“{{ drawTarget.name }}”将立即结束报名并按服务规则生成随机奖与保底奖，随后发起真实额度发放。</p></div><div class="dialog-actions"><button class="btn btn-secondary" type="button" :disabled="drawing" @click="drawTarget = null">暂不开奖</button><button class="btn btn-primary" type="button" :disabled="drawing" @click="drawCampaign"><RefreshCw v-if="drawing" class="spin" :size="16" /><ChartNoAxesColumnIncreasing v-else :size="16" />{{ drawing ? '开奖中' : '确认开奖' }}</button></div></div></div>

    <div v-if="cancelTarget" class="overlay" @click.self="cancelTarget = null"><div class="dialog confirm-dialog"><div class="dialog-body"><span class="danger-icon"><Ban :size="23" /></span><h2>取消这期活动？</h2><p>“{{ cancelTarget.name }}”取消后将立即停止报名，不开奖也不发放任何额度。该操作不能恢复。</p></div><div class="dialog-actions"><button class="btn btn-secondary" type="button" @click="cancelTarget = null">暂不取消</button><button class="btn btn-danger" type="button" @click="cancelCampaign">确认取消</button></div></div></div>

    <div v-if="toast" class="toast"><CircleCheckBig :size="18" />{{ toast }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  ArrowRight, Ban, Calculator, CalendarClock, CalendarRange, ChartNoAxesColumnIncreasing,
  ChevronLeft, ChevronRight, CircleAlert, CircleCheckBig, Copy, Download, ExternalLink, Eye,
  LockKeyhole, LogOut, Pencil, Plus, Radio, ReceiptText, RefreshCw, Save, Search,
  ShieldCheck, UsersRound, WalletCards, X, Mail
} from 'lucide-vue-next'
import { formatAmount, formatFullDate, scaledBudget } from '@shared/format'
import { ApiError, apiRequest } from '@shared/api'
import type { Campaign, CampaignStatus, PayoutRecord } from '@shared/types'
import logoUrl from '@shared/togoapi-logo.png'

const loggedIn = ref(Boolean(window.localStorage.getItem('portal-access-token')))
const adminEmail = ref('')
const password = ref('')
const loggingIn = ref(false)
const page = ref<'campaigns' | 'payouts'>('campaigns')
const campaignList = ref<Campaign[]>([])
const records = ref<PayoutRecord[]>([])
const dataLoading = ref(false)
const dataError = ref('')
const saving = ref(false)
const retryingId = ref<number | null>(null)
const campaignSearch = ref('')
const campaignStatus = ref('all')
const recordSearch = ref('')
const recordType = ref('all')
const recordStatus = ref('all')
const drawerOpen = ref(false)
const editingId = ref<number | null>(null)
const drawTarget = ref<Campaign | null>(null)
const drawing = ref(false)
const cancelTarget = ref<Campaign | null>(null)
const toast = ref('')
const nowTick = ref(Date.now())
let toastTimer = 0
let clockTimer = 0

const blankForm = (): Campaign => ({ id: Date.now(), name: '', subtitle: '', registrationStart: '', drawAt: '', participantLimit: 200, randomLimit: 100, randomMin: 5, randomMax: 20, randomBudget: 1200, guaranteeAmount: 1, participants: 0, randomWinners: 0, guaranteedWinners: 0, credited: 0, failed: 0, status: 'draft', published: false })
const form = reactive<Campaign>(blankForm())

const failedCount = computed(() => records.value.filter(item => item.status === 'failed').length)
const activeCampaign = computed(() => campaignList.value.find(item => item.published && ['scheduled', 'registering', 'drawing'].includes(item.status)) ?? null)
const activeEstimatedBudget = computed(() => activeCampaign.value ? scaledBudget(activeCampaign.value, activeCampaign.value.participants) : 0)
const activeProgress = computed(() => activeCampaign.value ? Math.min(100, activeCampaign.value.participants / Math.max(1, activeCampaign.value.participantLimit) * 100) : 0)
const drawDistance = computed(() => {
  if (!activeCampaign.value) return '--'
  const remaining = new Date(activeCampaign.value.drawAt).getTime() - nowTick.value
  if (remaining <= 0) return activeCampaign.value.status === 'drawing' ? '开奖中' : '已到开奖时间'
  const days = Math.floor(remaining / 86400000)
  const hours = Math.floor((remaining % 86400000) / 3600000)
  const minutes = Math.floor((remaining % 3600000) / 60000)
  return days > 0 ? `${days}天 ${hours}时` : `${hours}时 ${minutes}分`
})
const creditedRecords = computed(() => records.value.filter(item => item.status === 'credited'))
const creditedAmount = computed(() => creditedRecords.value.reduce((sum, item) => sum + item.amount, 0))
const randomCreditedAmount = computed(() => creditedRecords.value.filter(item => item.prizeType === 'random').reduce((sum, item) => sum + item.amount, 0))
const guaranteeCreditedAmount = computed(() => creditedRecords.value.filter(item => item.prizeType === 'guarantee').reduce((sum, item) => sum + item.amount, 0))
const creditedCount = computed(() => creditedRecords.value.length)
const processingCount = computed(() => records.value.filter(item => item.status === 'processing' || item.status === 'pending').length)
const payoutSuccessRate = computed(() => records.value.length ? (creditedCount.value / records.value.length * 100).toFixed(1) : '0.0')
const filteredCampaigns = computed(() => campaignList.value.filter(item => (campaignStatus.value === 'all' || item.status === campaignStatus.value) && item.name.toLowerCase().includes(campaignSearch.value.toLowerCase())))
const filteredRecords = computed(() => records.value.filter(item => (recordType.value === 'all' || item.prizeType === recordType.value) && (recordStatus.value === 'all' || item.status === recordStatus.value) && item.email.toLowerCase().includes(recordSearch.value.toLowerCase())))
const formLocked = computed(() => editingId.value !== null && ['registering', 'drawing', 'completed'].includes(form.status))
const averageBudget = computed(() => formatAmount(Number(form.randomBudget || 0) / Math.max(1, Number(form.randomLimit || 1))))
const budgetValid = computed(() => Number(averageBudget.value) >= Number(form.randomMin) && Number(averageBudget.value) <= Number(form.randomMax) && Number(form.randomMin) <= Number(form.randomMax))
const currentScaledBudget = computed(() => formatAmount(scaledBudget(form, editingId.value ? form.participants : Math.round(form.randomLimit * .7))))

async function login() {
  loggingIn.value = true
  try {
    const result = await apiRequest<{ access_token: string; user: { role: string } }>('/api/v1/auth/login', { method: 'POST', body: JSON.stringify({ identifier: adminEmail.value, password: password.value }) })
    if (result.user?.role !== 'admin') throw new Error('当前账户不是管理员')
    window.localStorage.setItem('portal-access-token', result.access_token)
    loggedIn.value = true
    password.value = ''
    showToast('登录成功')
    await loadRealData()
  } catch (error) {
    showToast(errorMessage(error, '登录失败'))
  } finally {
    loggingIn.value = false
  }
}
function logout() { window.localStorage.removeItem('portal-access-token'); loggedIn.value = false; campaignList.value = []; records.value = [] }
function assignForm(source: Campaign) { Object.assign(form, JSON.parse(JSON.stringify(source))); form.registrationStart = toLocalInput(source.registrationStart); form.drawAt = toLocalInput(source.drawAt) }
function openCreate() { editingId.value = null; assignForm(blankForm()); drawerOpen.value = true }
function openEdit(campaign: Campaign) { editingId.value = campaign.id; assignForm(campaign); drawerOpen.value = true }
function closeDrawer() { drawerOpen.value = false }
async function saveCampaign() {
  const saved = { ...form, registrationStart: new Date(form.registrationStart).toISOString(), drawAt: new Date(form.drawAt).toISOString() }
  const payload = { name: saved.name, subtitle: saved.subtitle, registration_start: saved.registrationStart, draw_at: saved.drawAt, participant_limit: saved.participantLimit, random_limit: saved.randomLimit, random_min: saved.randomMin, random_max: saved.randomMax, random_budget: saved.randomBudget, guarantee_amount: saved.guaranteeAmount, published: saved.published }
  saving.value = true
  try {
    if (editingId.value) await apiRequest(`/api/v1/admin/lottery/campaigns/${editingId.value}`, { method: 'PUT', body: JSON.stringify(payload) })
    else await apiRequest('/api/v1/admin/lottery/campaigns', { method: 'POST', body: JSON.stringify(payload) })
    const message = editingId.value ? '活动配置已保存' : '新活动已创建'
    await loadRealData()
    closeDrawer()
    showToast(message)
  } catch (error) {
    showToast(errorMessage(error, '保存活动失败'))
  } finally {
    saving.value = false
  }
}
function confirmDraw(campaign: Campaign) { drawTarget.value = campaign }
async function drawCampaign() {
  if (!drawTarget.value) return
  drawing.value = true
  try {
    await apiRequest(`/api/v1/admin/lottery/campaigns/${drawTarget.value.id}/draw`, { method: 'POST' })
    drawTarget.value = null
    await loadRealData()
    showToast('开奖完成，额度发放已进入服务队列')
  } catch (error) {
    showToast(errorMessage(error, '开奖失败'))
  } finally {
    drawing.value = false
  }
}
function confirmCancel(campaign: Campaign) { cancelTarget.value = campaign }
async function cancelCampaign() {
  if (!cancelTarget.value) return
  try {
    await apiRequest(`/api/v1/admin/lottery/campaigns/${cancelTarget.value.id}`, { method: 'DELETE' })
    cancelTarget.value = null
    await loadRealData()
    showToast('活动已取消，报名入口已关闭')
  } catch (error) {
    showToast(errorMessage(error, '取消活动失败'))
  }
}
function duplicateCampaign(campaign: Campaign) {
  editingId.value = null
  assignForm({ ...campaign, id: 0, name: `${campaign.name}（副本）`, status: 'draft', published: false, participants: 0, randomWinners: 0, guaranteedWinners: 0, credited: 0, failed: 0 })
  drawerOpen.value = true
}
async function retryRecord(record: PayoutRecord) {
  retryingId.value = record.id
  try {
    await apiRequest(`/api/v1/admin/lottery/payouts/${record.id}/retry`, { method: 'POST' })
    await loadRealData()
    showToast(`记录 #${record.id} 已进入重试队列`)
  } catch (error) {
    showToast(errorMessage(error, '重试失败'))
  } finally {
    retryingId.value = null
  }
}
function exportRecords() {
  if (!filteredRecords.value.length) { showToast('当前没有可导出的记录'); return }
  const rows = [['参与账户', '所属活动', '奖项', '中奖额度', '发放时间', '状态'], ...filteredRecords.value.map(item => [item.email, item.campaign, item.prizeType === 'random' ? '随机奖' : '保底奖', formatAmount(item.amount), formatFullDate(item.createdAt), payoutText(item.status)])]
  const csv = '\uFEFF' + rows.map(row => row.map(value => `"${String(value).replace(/"/g, '""')}"`).join(',')).join('\r\n')
  const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
  const link = document.createElement('a'); link.href = url; link.download = `lottery-payouts-${new Date().toISOString().slice(0, 10)}.csv`; link.click(); URL.revokeObjectURL(url)
}
function showToast(message: string) { toast.value = message; window.clearTimeout(toastTimer); toastTimer = window.setTimeout(() => { toast.value = '' }, 3000) }
function toLocalInput(value: string) { if (!value) return ''; const date = new Date(value); if (Number.isNaN(date.getTime())) return ''; const offset = date.getTimezoneOffset() * 60000; return new Date(date.getTime() - offset).toISOString().slice(0, 16) }
function statusText(status: CampaignStatus) { return ({ draft: '草稿', scheduled: '待开始', registering: '报名中', drawing: '开奖中', completed: '已开奖', cancelled: '已取消' })[status] }
function statusClass(status: CampaignStatus) { return ({ draft: 'badge-gray', scheduled: 'badge-blue', registering: 'badge-green', drawing: 'badge-amber', completed: 'badge-blue', cancelled: 'badge-red' })[status] }
function payoutText(status: PayoutRecord['status']) { return ({ pending: '待处理', credited: '已到账', processing: '处理中', failed: '失败' })[status] }
function payoutClass(status: PayoutRecord['status']) { return ({ pending: 'badge-gray', credited: 'badge-green', processing: 'badge-blue', failed: 'badge-red' })[status] }
function errorMessage(error: unknown, fallback: string) { return error instanceof Error ? error.message : fallback }

function mapCampaign(data: Record<string, unknown>): Campaign {
  return {
    ...blankForm(), id: Number(data.id), name: String(data.name || ''), subtitle: String(data.subtitle || ''),
    registrationStart: String(data.registration_start || ''), drawAt: String(data.draw_at || ''),
    participantLimit: Number(data.participant_limit || 0), randomLimit: Number(data.random_limit || 0),
    randomMin: Number(data.random_min || 0), randomMax: Number(data.random_max || 0), randomBudget: Number(data.random_budget || 0), guaranteeAmount: Number(data.guarantee_amount || 0),
    participants: Number(data.participants || 0), randomWinners: Number(data.random_winners || 0), guaranteedWinners: Number(data.guaranteed_winners || 0), credited: Number(data.credited || 0), failed: Number(data.failed || 0),
    status: (data.status || 'draft') as CampaignStatus, published: Boolean(data.published), randomPrize: Number(data.random_prize || 0), guaranteePrize: Number(data.guarantee_prize || 0)
  }
}

async function loadRealData() {
  dataLoading.value = true
  dataError.value = ''
  try {
    const [campaignItems, payoutItems] = await Promise.all([
      apiRequest<Record<string, unknown>[]>('/api/v1/admin/lottery/campaigns'),
      apiRequest<Record<string, unknown>[]>('/api/v1/admin/lottery/payouts')
    ])
    campaignList.value = campaignItems.map(mapCampaign)
    records.value = payoutItems.map(item => ({ id: Number(item.id), campaign: String(item.campaign_name || ''), email: String(item.email || ''), prizeType: (item.prize_type || 'random') as PayoutRecord['prizeType'], amount: Number(item.amount || 0), status: (item.status || 'pending') as PayoutRecord['status'], createdAt: String(item.created_at || '') }))
  } catch (error) {
    dataError.value = errorMessage(error, '无法获取服务数据')
    if (error instanceof ApiError && error.status === 401) logout()
  } finally {
    dataLoading.value = false
  }
}

onMounted(() => {
  if (window.localStorage.getItem('portal-access-token')) { loggedIn.value = true; loadRealData() }
  clockTimer = window.setInterval(() => { nowTick.value = Date.now() }, 30000)
})
onBeforeUnmount(() => { window.clearTimeout(toastTimer); window.clearInterval(clockTimer) })
</script>
