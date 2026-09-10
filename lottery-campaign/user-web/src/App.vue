<template>
  <div class="user-page">
    <header class="site-header">
      <div class="shell header-inner">
        <a class="brand" href="#" aria-label="TogoAPI 首页">
          <img :src="logoUrl" alt="TogoAPI" />
          <span class="brand-copy"><strong>TogoAPI</strong><span>额度惊喜计划</span></span>
        </a>
        <button class="history-link" type="button" @click="scrollToHistory">
          <History :size="17" />
          往期活动
        </button>
      </div>
    </header>

    <main>
      <section v-if="current" class="campaign-band">
        <div class="campaign-particles" aria-hidden="true">
          <i v-for="particle in heroParticles" :key="particle.id" :style="particle.style"></i>
        </div>
        <div class="shell campaign-inner">
          <div class="campaign-copy">
            <span class="live-pill" :class="{ completed: isCompleted }"><span></span> {{ currentStatusLabel }}</span>
            <h1>{{ current.name }}</h1>
            <p class="campaign-subtitle">{{ current.subtitle }}</p>
            <div class="campaign-meta">
              <div><CalendarDays :size="17" /><span>报名开始</span><strong>{{ formatDate(current.registrationStart) }}</strong></div>
              <div><Sparkles :size="17" /><span>自动开奖</span><strong>{{ formatDate(current.drawAt) }}</strong></div>
            </div>
            <div class="integrated-countdown" :class="{ completed: isCompleted }" :aria-label="isCompleted ? '本期活动已结束' : '开奖倒计时'">
              <div class="countdown-title">
                <span class="clock-pulse"><CircleCheckBig v-if="isCompleted" :size="20" /><Clock3 v-else :size="20" /></span>
                <span><strong>{{ isCompleted ? '本期活动已结束' : '距离自动开奖' }}</strong><small>{{ isCompleted ? '惊喜额度已自动发放至账户' : '到点自动开奖并发放额度' }}</small></span>
              </div>
              <div v-if="!isCompleted" class="countdown-values">
                <div class="time-unit"><Transition name="digit-roll" mode="out-in"><strong :key="countdown.days">{{ countdown.days }}</strong></Transition><span>天</span></div><i>:</i>
                <div class="time-unit"><Transition name="digit-roll" mode="out-in"><strong :key="countdown.hours">{{ countdown.hours }}</strong></Transition><span>时</span></div><i>:</i>
                <div class="time-unit"><Transition name="digit-roll" mode="out-in"><strong :key="countdown.minutes">{{ countdown.minutes }}</strong></Transition><span>分</span></div><i>:</i>
                <div class="time-unit seconds"><Transition name="digit-roll" mode="out-in"><strong :key="countdown.seconds">{{ countdown.seconds }}</strong></Transition><span>秒</span></div>
              </div>
              <div v-else class="campaign-completed-copy"><Sparkles :size="18" /><span>开奖结果已揭晓</span></div>
              <button class="result-link countdown-result-link" type="button" :disabled="!drawReady" @click="openCurrentResult">
                <Clock3 v-if="!drawReady" :size="15" />
                <Search v-else :size="15" />
                {{ drawReady ? '查看本期结果' : '开奖后查看' }}
              </button>
            </div>
          </div>

          <div class="hero-registration">
            <div class="signup-ribbon"><span></span>{{ canRegister ? '报名入口已开启' : current.status === 'scheduled' ? '报名尚未开始' : '报名已经截止' }}</div>
            <div class="registration-head">
              <div class="section-heading">
                <span class="section-icon"><TicketCheck :size="21" /></span>
                <div><h2>{{ isCompleted ? '开奖结果已揭晓' : registered ? '等待开奖' : canRegister ? '填写账户邮箱' : current.status === 'scheduled' ? '等待报名开放' : '等待开奖结果' }}</h2><p>{{ isCompleted ? '惊喜额度已自动发放至账户' : registered ? '报名成功，已为你保留本期参与资格' : canRegister ? '使用已注册的 TogoAPI 账户邮箱' : '活动状态由服务实时同步' }}</p></div>
              </div>
            </div>

            <Transition name="state-swap" mode="out-in">
              <form v-if="!registered" key="signup" class="signup-form" @submit.prevent="submitRegistration">
                <div class="field">
                  <label for="email">账户邮箱</label>
                  <div class="email-input-wrap">
                    <Mail :size="19" />
                    <input id="email" v-model="email" class="input" :class="{ 'input-error': errorMessage }" type="email" autocomplete="email" placeholder="name@example.com" :disabled="!canRegister" required @input="errorMessage = ''" />
                  </div>
                  <p v-if="errorMessage" class="form-error" aria-live="polite"><CircleAlert :size="14" />{{ errorMessage }}</p>
                </div>
                <label class="agreement-row">
                  <input v-model="agreed" type="checkbox" :disabled="!canRegister" />
                  <span>同意本期活动规则</span>
                </label>
                <button class="btn btn-primary signup-button" type="submit" :disabled="!canRegister || !agreed || submitting">
                  <LoaderCircle v-if="submitting" class="spin" :size="18" />
                  <Gift v-else :size="18" />
                  {{ submitting ? '正在确认账户' : canRegister ? '立即报名' : current.status === 'scheduled' ? '尚未开始' : '报名已截止' }}
                </button>
              </form>

              <div v-else key="registered" class="registered-state" aria-live="polite">
                <div class="success-seal"><Check :size="28" stroke-width="2.5" /></div>
                <div class="registered-copy"><span>{{ isCompleted ? '本期已开奖 · 参与账户' : '等待开奖 · 已报名账户' }}</span><strong>{{ email }}</strong><p>{{ isCompleted ? '点击上方“查看本期结果”，可再次查看惊喜额度。' : '开奖后进入本页面即可查看惊喜额度。' }}</p></div>
                <button v-if="!isCompleted" class="btn btn-secondary btn-sm" type="button" @click="clearRegistration">更换账户</button>
              </div>
            </Transition>
          </div>
        </div>
      </section>

      <section v-else class="campaign-band campaign-service-state">
        <div class="shell service-message">
          <RefreshCw v-if="currentLoading" class="spin" :size="28" />
          <CalendarDays v-else :size="28" />
          <div><h1>{{ currentLoading ? '正在加载活动' : currentError ? '活动服务暂不可用' : '当前暂无开放活动' }}</h1><p>{{ currentLoading ? '正在从服务同步最新活动信息' : currentError || '新一期发布后即可在这里报名参与' }}</p></div>
          <button v-if="!currentLoading && currentError" class="btn btn-secondary" type="button" @click="loadCurrent"><RefreshCw :size="16" />重新加载</button>
        </div>
      </section>

      <section v-if="current" class="compact-rules">
        <div class="shell compact-rules-inner">
          <div><ShieldCheck :size="18" /><span><strong>注册邮箱报名</strong>每期仅限一次</span></div>
          <div><Sparkles :size="18" /><span><strong>系统自动开奖</strong>无需手动领取</span></div>
          <div><Gift :size="18" /><span><strong>惊喜额度到账</strong>每位参与者均可获得</span></div>
          <div class="participants-row"><UsersRound :size="17" /><span>已有 <strong>{{ current.participants }}</strong> 人报名</span></div>
        </div>
      </section>

      <section v-if="!current || isCompleted" class="account-results-section">
        <div class="shell">
          <div class="account-results-panel">
            <div class="account-results-heading">
              <span class="section-icon"><Search :size="20" /></span>
              <div><h2>查看我的往期开奖结果</h2><p>输入报名邮箱，查看这个账户参加过的历史活动记录</p></div>
            </div>
            <form class="account-results-form" @submit.prevent="lookupAccountResults">
              <div class="email-input-wrap">
                <Mail :size="18" />
                <input v-model="lookupEmail" class="input" type="email" autocomplete="email" placeholder="请输入报名时使用的账户邮箱" :disabled="lookupLoading" required />
              </div>
              <button class="btn btn-primary" type="submit" :disabled="lookupLoading || !lookupEmail.trim()">
                <LoaderCircle v-if="lookupLoading" class="spin" :size="17" />
                <Search v-else :size="17" />
                {{ lookupLoading ? '正在查询' : '查看我的记录' }}
              </button>
            </form>
            <p v-if="lookupError" class="account-results-error"><CircleAlert :size="15" />{{ lookupError }}</p>
            <div v-if="lookupDone && !lookupLoading && !lookupError && accountResults.length === 0" class="account-results-empty">
              <Search :size="20" /><span>暂未找到该邮箱的已开奖记录</span>
            </div>
            <div v-if="accountResults.length" class="account-results-list">
              <div class="account-results-summary"><strong>{{ lookupEmail }}</strong><span>共 {{ accountResults.length }} 期已开奖记录</span></div>
              <article v-for="item in accountResults" :key="item.id" class="account-result-item">
                <div class="account-result-date"><strong>{{ formatHistoryDay(item.createdAt) }}</strong><small>{{ formatHistoryMonth(item.createdAt) }}</small></div>
                <div class="account-result-info"><strong>{{ item.campaignName }}</strong><span>{{ formatDate(item.createdAt) }}</span></div>
                <div class="account-result-prize" :class="item.prizeType === 'guarantee' ? 'guarantee' : ''"><span>{{ item.prizeType === 'guarantee' ? '保底奖' : '随机惊喜奖' }}</span><strong>{{ formatAmount(item.amount) }}</strong></div>
              </article>
            </div>
          </div>
        </div>
      </section>

      <section ref="historySection" class="history-section">
        <div class="shell">
          <div class="history-header">
            <div class="section-heading history-heading">
              <span class="section-icon amber"><History :size="21" /></span>
              <div><h2>往期活动</h2><p>默认展示最新一期真实发放汇总，查看更多历史记录</p></div>
            </div>
            <button class="history-more" type="button" @click="toggleHistory">
              <ChevronDown :size="16" :class="{ rotated: historyExpanded }" />
              {{ historyExpanded ? '收起记录' : `查看更多（${historyTotal}期）` }}
            </button>
          </div>
          <div class="history-list">
            <div v-if="historyLoading" class="history-state"><RefreshCw class="spin" :size="18" />正在加载往期活动</div>
            <div v-else-if="historyError" class="history-state error"><CircleAlert :size="18" />{{ historyError }}<button class="btn btn-secondary btn-sm" type="button" @click="loadHistory(1)"><RefreshCw :size="14" />重试</button></div>
            <div v-else-if="visibleHistory.length === 0" class="history-state">暂无往期活动</div>
            <article v-for="item in visibleHistory" :key="item.id" class="past-card">
              <div class="past-card-main">
                <div class="past-date"><span>{{ formatHistoryDay(item.drawAt) }}</span><small>{{ formatHistoryMonth(item.drawAt) }}</small></div>
                <div class="past-info">
                  <span class="badge badge-gray">已开奖</span>
                  <h3>{{ item.name }}</h3>
                  <p>{{ item.participants }} 人参与 · {{ item.randomWinners + item.guaranteedWinners }} 人中奖</p>
                  <div class="past-prizes">
                    <span><strong>{{ item.randomWinners }}人 / {{ formatAmount(item.randomPrize) }}</strong> 随机奖发放</span>
                    <span><strong>{{ item.guaranteedWinners }}人 / {{ formatAmount(item.guaranteePrize) }}</strong> 保底奖发放</span>
                  </div>
                </div>
              </div>
            </article>
          </div>
          <div v-if="historyExpanded" class="history-pagination">
            <span>第 {{ historyPage }} / {{ historyPageCount }} 页</span>
            <div>
              <button class="btn btn-ghost btn-icon" type="button" aria-label="上一页" :disabled="historyPage === 1" @click="changeHistoryPage(-1)"><ChevronLeft :size="17" /></button>
              <button class="btn btn-ghost btn-icon" type="button" aria-label="下一页" :disabled="historyPage === historyPageCount" @click="changeHistoryPage(1)"><ChevronRight :size="17" /></button>
            </div>
          </div>
        </div>
      </section>
    </main>

    <footer><div class="shell"><span>© 2026 TogoAPI</span><span>额度发放以账户记录为准</span></div></footer>

    <div v-if="resultOpen" class="result-overlay" role="dialog" aria-modal="true" aria-label="中奖结果">
      <div v-if="resultFirstView" class="confetti" aria-hidden="true">
        <i v-for="piece in confetti" :key="piece.id" :style="piece.style"></i>
      </div>
      <button class="result-close" type="button" aria-label="关闭结果" @click="closeResultAndRefresh"><X :size="20" /></button>
      <div class="result-content">
        <div class="result-rays" aria-hidden="true"></div>
        <div class="result-gift" :class="{ static: !resultFirstView }"><Gift :size="46" /></div>
        <p class="result-kicker">惊喜时刻</p>
        <h2>恭喜获得{{ resultPrizeLabel }}</h2>
        <p class="result-account">{{ resultAccount }}</p>
        <div class="amount-reveal"><span>{{ resultPrizeLabel }}</span><strong>{{ formatAmount(resultAmount || 0) }}</strong><small>额度</small></div>
        <p class="result-note"><CircleCheckBig :size="16" />额度已自动发放至你的账户</p>
        <button class="result-action" type="button" @click="closeResultAndRefresh">收下惊喜 <ArrowRight :size="18" /></button>
      </div>
    </div>

    <div v-if="registrationModal" class="registration-overlay" role="dialog" aria-modal="true" aria-label="报名提示">
      <div class="registration-burst" aria-hidden="true">
        <i v-for="piece in registrationBurst" :key="piece.id" :style="piece.style"></i>
      </div>
      <button class="result-close registration-close" type="button" aria-label="关闭提示" @click="closeRegistrationModal"><X :size="20" /></button>
      <div class="registration-modal-card" :class="{ duplicate: registrationModal === 'duplicate' }">
        <div class="registration-modal-icon"><Sparkles v-if="registrationModal === 'success'" :size="38" /><Check v-else :size="38" stroke-width="3" /></div>
        <p class="registration-modal-kicker">{{ registrationModal === 'success' ? '报名成功' : '已参与本期活动' }}</p>
        <h2>{{ registrationModal === 'success' ? '惊喜资格已锁定！' : '你已经报名过啦' }}</h2>
        <p class="registration-modal-copy">{{ registrationModal === 'success' ? '恭喜你，已成功加入本期 TogoAPI 惊喜活动。' : '同一个账户每期只能参与一次，无需重复报名。' }}</p>
        <div v-if="registrationModal === 'success' && current" class="draw-reminder">
          <CalendarDays :size="22" />
          <span><small>开奖时间</small><strong>{{ formatDate(current.drawAt) }}</strong></span>
        </div>
        <div v-else class="draw-reminder duplicate-reminder">
          <Clock3 :size="22" />
          <span><small>请耐心等待</small><strong>开奖后回来查看惊喜额度</strong></span>
        </div>
        <p class="registration-modal-note">{{ registrationModal === 'success' ? '开奖后将自动揭晓额度，请记得回来查看。' : '当前页面已为你切换到等待开奖状态。' }}</p>
        <button class="registration-modal-action" type="button" @click="closeRegistrationModal">{{ registrationModal === 'success' ? '知道了，等待开奖' : '好的，等待开奖' }} <ArrowRight :size="18" /></button>
      </div>
    </div>

    <div v-if="toast" class="toast"><CircleCheckBig :size="18" />{{ toast }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  ArrowRight, CalendarDays, Check, ChevronDown, ChevronLeft, ChevronRight, CircleAlert,
  CircleCheckBig, Clock3, Gift, History, LoaderCircle, Mail, RefreshCw, Search, ShieldCheck, Sparkles,
  TicketCheck, UsersRound, X
} from 'lucide-vue-next'
import { formatAmount, formatDate, maskEmail } from '@shared/format'
import { ApiError, apiRequest } from '@shared/api'
import type { Campaign } from '@shared/types'
import logoUrl from '@shared/togoapi-logo.png'

type HistoryCampaign = {
  id: number
  name: string
  drawAt: string
  participants: number
  randomWinners: number
  guaranteedWinners: number
  randomPrize: number
  guaranteePrize: number
}

type AccountResult = {
  id: number
  campaignId: number
  campaignName: string
  email: string
  prizeType: 'random' | 'guarantee'
  amount: number
  status: string
  createdAt: string
}

const current = ref<Campaign | null>(null)
const latestCompleted = ref<Campaign | null>(null)
const historyCampaigns = ref<HistoryCampaign[]>([])
const currentLoading = ref(true)
const currentError = ref('')
const historyLoading = ref(true)
const historyError = ref('')
const historyTotal = ref(0)
const email = ref('')
const lookupEmail = ref('')
const accountResults = ref<AccountResult[]>([])
const lookupLoading = ref(false)
const lookupError = ref('')
const lookupDone = ref(false)
const agreed = ref(false)
const submitting = ref(false)
const registered = ref(false)
const errorMessage = ref('')
const registrationModal = ref<'success' | 'duplicate' | null>(null)
const resultOpen = ref(false)
const resultFirstView = ref(true)
const resultAmount = ref<number | null>(null)
const resultPrizeLabel = ref('随机惊喜奖')
const resultAccount = ref('本期中奖结果')
const toast = ref('')
const historySection = ref<HTMLElement | null>(null)
const historyExpanded = ref(false)
const historyPage = ref(1)
const tick = ref(Date.now())
let timer = 0
let toastTimer = 0
let participantRefreshTimer = 0
let participantsRefreshing = false

const canRegister = computed(() => current.value?.status === 'registering')
const isCompleted = computed(() => current.value?.status === 'completed')
const currentStatusLabel = computed(() => {
  if (current.value?.status === 'completed') return '本期已开奖'
  if (current.value?.status === 'scheduled') return '即将开放报名'
  if (current.value?.status === 'drawing') return '正在开奖'
  return '报名进行中'
})
const countdownRemaining = computed(() => current.value ? Math.max(0, new Date(current.value.drawAt).getTime() - tick.value) : 0)
const drawReady = computed(() => countdownRemaining.value <= 0)
const countdown = computed(() => {
  const remaining = countdownRemaining.value
  const days = Math.floor(remaining / 86400000)
  const hours = Math.floor((remaining % 86400000) / 3600000)
  const minutes = Math.floor((remaining % 3600000) / 60000)
  const seconds = Math.floor((remaining % 60000) / 1000)
  return { days: String(days).padStart(2, '0'), hours: String(hours).padStart(2, '0'), minutes: String(minutes).padStart(2, '0'), seconds: String(seconds).padStart(2, '0') }
})
const historyPageSize = 2
const historyPageCount = computed(() => Math.max(1, Math.ceil(historyTotal.value / historyPageSize)))
const visibleHistory = computed(() => {
  if (!historyExpanded.value) return historyCampaigns.value.slice(0, 1)
  return historyCampaigns.value
})

const colors = ['#fbbf24', '#f9a8d4', '#db2777', '#ffffff', '#0f766e']
const confetti = Array.from({ length: 54 }, (_, id) => ({
  id,
  style: {
    left: `${(id * 37) % 100}%`,
    background: colors[id % colors.length],
    animationDelay: `${(id % 11) * 0.08}s`,
    animationDuration: `${2.4 + (id % 7) * 0.18}s`,
    transform: `rotate(${id * 29}deg)`
  }
}))

const registrationBurst = Array.from({ length: 28 }, (_, id) => ({
  id,
  style: {
    '--burst-x': `${((id * 53) % 360) - 180}px`,
    '--burst-y': `${-80 - ((id * 37) % 190)}px`,
    '--piece-delay': `${(id % 7) * 0.035}s`,
    '--piece-angle': `${id * 23}deg`
  }
}))

const heroParticles = Array.from({ length: 14 }, (_, id) => ({
  id,
  style: {
    left: `${5 + ((id * 19) % 91)}%`,
    top: `${10 + ((id * 31) % 78)}%`,
    animationDelay: `${(id % 7) * .55}s`,
    animationDuration: `${4.8 + (id % 5) * .65}s`
  }
}))

function mapAccountResult(data: Record<string, unknown>): AccountResult {
  return {
    id: Number(data.id || 0), campaignId: Number(data.campaign_id || 0), campaignName: String(data.campaign_name || ''),
    email: String(data.email || ''), prizeType: data.prize_type === 'guarantee' ? 'guarantee' : 'random', amount: Number(data.amount || 0),
    status: String(data.status || ''), createdAt: String(data.created_at || '')
  }
}

async function lookupAccountResults() {
  const normalizedEmail = lookupEmail.value.trim().toLowerCase()
  if (!normalizedEmail) return
  lookupLoading.value = true
  lookupError.value = ''
  lookupDone.value = false
  accountResults.value = []
  try {
    const data = await apiRequest<{ items: Record<string, unknown>[] }>('/api/v1/lottery/results?email=' + encodeURIComponent(normalizedEmail))
    lookupEmail.value = normalizedEmail
    window.localStorage.setItem('lottery-result-email', normalizedEmail)
    accountResults.value = data.items.map(mapAccountResult)
    lookupDone.value = true
  } catch (error) {
    lookupError.value = error instanceof Error ? error.message : '查询失败，请稍后重试'
  } finally {
    lookupLoading.value = false
  }
}

async function submitRegistration() {
  errorMessage.value = ''
  if (!current.value || !canRegister.value) return
  submitting.value = true
  const normalizedEmail = email.value.trim().toLowerCase()
  try {
    const campaignId = current.value.id
    await apiRequest<{ email: string }>('/api/v1/lottery/register', { method: 'POST', body: JSON.stringify({ email: normalizedEmail }) })
    registered.value = true
    email.value = normalizedEmail
    window.localStorage.setItem('lottery-current-registration', JSON.stringify({ campaignId, email: normalizedEmail }))
    if (current.value?.id === campaignId) current.value.participants += 1
    void refreshParticipantCount()
    registrationModal.value = 'success'
  } catch (error) {
    if (error instanceof ApiError && error.code === 'ALREADY_REGISTERED') {
      registered.value = true
      email.value = normalizedEmail
      window.localStorage.setItem('lottery-current-registration', JSON.stringify({ campaignId: current.value.id, email: normalizedEmail }))
      errorMessage.value = ''
      registrationModal.value = 'duplicate'
    } else {
      errorMessage.value = error instanceof Error ? error.message : '报名失败，请稍后重试'
    }
  } finally {
    submitting.value = false
  }
}

function closeRegistrationModal() {
  registrationModal.value = null
}

function openCurrentResult() {
  if (!drawReady.value) return
  if (!registered.value || !email.value) {
    showToast('请先报名，开奖后即可查看本期结果')
    return
  }
  const campaign = current.value
  if (!campaign) return
  apiRequest<{ amount: number; prize_type: string; email: string }>('/api/v1/lottery/current/result?email=' + encodeURIComponent(email.value.trim()))
    .then(result => {
      const revealKey = `lottery-revealed-${campaign.id}-${email.value.trim().toLowerCase()}`
      resultFirstView.value = window.localStorage.getItem(revealKey) !== 'true'
      window.localStorage.setItem(revealKey, 'true')
      resultAmount.value = result.amount
      resultPrizeLabel.value = result.prize_type === 'guarantee' ? '保底奖' : '随机惊喜奖'
      resultAccount.value = email.value
      resultOpen.value = true
    })
    .catch(error => showToast(error instanceof Error ? error.message : '结果暂不可用'))
}

function closeResultAndRefresh() {
  resultOpen.value = false
  if (current.value) {
    current.value.status = 'completed'
    current.value.published = false
  }
  void loadHistory(1)
}

function toggleHistory() {
  historyExpanded.value = !historyExpanded.value
  historyPage.value = 1
  if (historyExpanded.value && historyCampaigns.value.length < historyTotal.value) loadHistory(1)
}

function changeHistoryPage(delta: number) {
  historyPage.value = Math.min(Math.max(historyPage.value + delta, 1), historyPageCount.value)
  if (historyExpanded.value) loadHistory(historyPage.value)
}

function formatHistoryDay(value: string) {
  return String(new Date(value).getDate()).padStart(2, '0')
}

function formatHistoryMonth(value: string) {
  const date = new Date(value)
  return `${date.getFullYear()}.${String(date.getMonth() + 1).padStart(2, '0')}`
}

function clearRegistration() {
  registered.value = false
  email.value = ''
  agreed.value = false
  window.localStorage.removeItem('lottery-current-registration')
}

function scrollToHistory() {
  historySection.value?.scrollIntoView({ behavior: 'smooth' })
}


function showToast(message: string) {
  toast.value = message
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => { toast.value = '' }, 3200)
}

async function loadCurrent() {
  currentLoading.value = true
  currentError.value = ''
  try {
    const data = await apiRequest<Record<string, unknown>>('/api/v1/lottery/current')
    current.value = mapCampaign(data)
    const saved = parseSavedRegistration()
    if (saved && saved.campaignId === current.value.id) {
      email.value = saved.email; registered.value = true; agreed.value = true
    }
  } catch (error) {
    current.value = null
    if (error instanceof ApiError && error.status === 404) {
      restoreCompletedCampaign()
    } else {
      currentError.value = error instanceof Error ? error.message : '无法加载当前活动'
    }
  } finally {
    currentLoading.value = false
  }
}

async function refreshParticipantCount() {
  const campaign = current.value
  if (!campaign || participantsRefreshing) return
  participantsRefreshing = true
  try {
    const data = await apiRequest<Record<string, unknown>>('/api/v1/lottery/current')
    const refreshed = mapCampaign(data)
    if (current.value?.id === refreshed.id) {
      current.value.participants = Math.max(current.value.participants, refreshed.participants)
    }
  } catch {
    // The main activity state remains usable when a background refresh fails.
  } finally {
    participantsRefreshing = false
  }
}

async function loadHistory(page = historyPage.value) {
  historyLoading.value = true
  historyError.value = ''
  try {
    const data = await apiRequest<{ items: Record<string, unknown>[]; total: number }>('/api/v1/lottery/history?page=' + page + '&page_size=' + historyPageSize)
    historyTotal.value = Number(data.total || 0)
    historyCampaigns.value = data.items.map(mapHistoryCampaign)
    latestCompleted.value = data.items.length > 0 ? mapCampaign(data.items[0]) : null
    if (!currentLoading.value && !currentError.value && !current.value) restoreCompletedCampaign()
  } catch (error) {
    historyError.value = error instanceof Error ? error.message : '无法加载往期活动'
    historyCampaigns.value = []
  } finally {
    historyLoading.value = false
  }
}

function restoreCompletedCampaign() {
  const saved = parseSavedRegistration()
  if (!saved || !latestCompleted.value || saved.campaignId !== latestCompleted.value.id) return
  current.value = { ...latestCompleted.value, status: 'completed', published: false }
  email.value = saved.email
  registered.value = true
  agreed.value = true
}

function parseSavedRegistration() {
  const raw = window.localStorage.getItem('lottery-current-registration')
  try {
    const value = JSON.parse(raw || '') as { campaignId?: number; email?: string }
    return value.campaignId && value.email ? { campaignId: Number(value.campaignId), email: value.email } : null
  } catch {
    if (raw) window.localStorage.removeItem('lottery-current-registration')
    return null
  }
}

onMounted(() => {
  lookupEmail.value = window.localStorage.getItem('lottery-result-email') || ''
  loadCurrent()
  loadHistory(1)
  timer = window.setInterval(() => { tick.value = Date.now() }, 1000)
  participantRefreshTimer = window.setInterval(refreshParticipantCount, 10000)
})
onBeforeUnmount(() => {
  window.clearInterval(timer)
  window.clearInterval(participantRefreshTimer)
  window.clearTimeout(toastTimer)
})

function mapCampaign(data: Record<string, unknown>) {
  return {
    id: Number(data.id), name: String(data.name || ''), subtitle: String(data.subtitle || ''),
    registrationStart: String(data.registration_start || data.registrationStart), drawAt: String(data.draw_at || data.drawAt),
    participantLimit: Number(data.participant_limit || data.participantLimit || 0), randomLimit: Number(data.random_limit || data.randomLimit || 0),
    randomMin: Number(data.random_min || data.randomMin || 0), randomMax: Number(data.random_max || data.randomMax || 0),
    randomBudget: Number(data.random_budget || data.randomBudget || 0), guaranteeAmount: Number(data.guarantee_amount || data.guaranteeAmount || 0),
    participants: Number(data.participants || 0), randomWinners: Number(data.random_winners || data.randomWinners || 0), guaranteedWinners: Number(data.guaranteed_winners || data.guaranteedWinners || 0),
    credited: Number(data.credited || 0), failed: Number(data.failed || 0), status: (data.status || 'registering') as Campaign['status'],
    published: Boolean(data.published), randomPrize: Number(data.random_prize || 0), guaranteePrize: Number(data.guarantee_prize || 0)
  }
}

function mapHistoryCampaign(data: Record<string, unknown>): HistoryCampaign {
  return {
    id: Number(data.id), name: String(data.name || ''), drawAt: String(data.draw_at || ''), participants: Number(data.participants || 0),
    randomWinners: Number(data.random_winners || 0), guaranteedWinners: Number(data.guaranteed_winners || 0),
    randomPrize: Number(data.random_prize || 0), guaranteePrize: Number(data.guarantee_prize || 0)
  }
}
</script>
