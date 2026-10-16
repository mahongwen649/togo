<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountCheckResult from '@/components/account/AccountCheckResult.vue'
import AccountCheckManagement from '@/components/account/AccountCheckManagement.vue'
import { listCheckAccounts, listCheckGroups, listCheckModels, runAccountCheck, cancelAccountCheck, listAccountCheckHistory, getAccountCheckSelection, saveAccountCheckSelection, type AccountCheckRecord, type CheckAccount, type CheckGroupOption, type AccountCheckResult as CheckResult } from '@/api/admin/accountChecks'

interface Selection {
  account: CheckAccount
  prompt: string
  model: string
  models: Array<{ id: string; display_name: string }>
  loadingModels: boolean
  modelError: string
}
interface Job {
  account: CheckAccount
  prompt: string
  model: string
  state: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  result: CheckResult | null
  error: string
  startedAt: number
  endedAt: number
}

const DEFAULT_CHECK_MODEL = 'gpt-6-astra'
const accounts = ref<CheckAccount[]>([]), selections = ref<Selection[]>([]), jobs = ref<Job[]>([])
const groups = ref<CheckGroupOption[]>([]), groupsLoading = ref(false)
const search = ref(''), page = ref(1), pages = ref(1), total = ref(0)
const loading = ref(false), selectingAll = ref(false), listError = ref('')
const running = computed(() => jobs.value.some(job => job.state === 'running' || job.state === 'queued'))
const histories = ref<Record<number, AccountCheckRecord[]>>({}), viewedHistory = ref<Record<number, number>>({})
const historyLoading = ref(true), historyError = ref(''), selectionSaving = ref(false)
const cancellingIDs = ref(new Set<number>())
let historyPoll: ReturnType<typeof setInterval> | undefined, pollingHistory = false
const mode = ref<'shared' | 'individual'>('shared'), sharedPrompt = ref(''), now = ref(Date.now())
let listController: AbortController | undefined, selectAllController: AbortController | undefined, groupsController: AbortController | undefined
const modelControllers = new Map<number, AbortController>(), runControllers = new Map<number, AbortController>()
let timer: ReturnType<typeof setInterval> | undefined, searchTimer: ReturnType<typeof setTimeout> | undefined
let disposed = false, activeWorkers = 0
const message = (e: unknown) => (e as { message?: string })?.message || '加载失败，请重试'
const selectedIDs = computed(() => new Set(selections.value.map(item => item.account.id)))
const validQuestions = computed(() => mode.value === 'shared' ? !!sharedPrompt.value.trim() : selections.value.every(item => !!item.prompt.trim()))
const canRun = computed(() => selections.value.length > 0 && validQuestions.value && !running.value && !selectingAll.value && !historyLoading.value && !selectionSaving.value)
const completed = computed(() => jobs.value.filter(job => !['queued', 'running'].includes(job.state)).length)
const succeeded = computed(() => jobs.value.filter(job => job.state === 'succeeded').length)
const failed = computed(() => jobs.value.filter(job => job.state === 'failed').length)
const accountJobs = computed(() => new Map(jobs.value.map(job => {
  const historical = histories.value[job.account.id]?.find(record => record.id === viewedHistory.value[job.account.id])
  return [job.account.id, [historical && !['queued', 'running'].includes(job.state) ? historyJob(historical) : job]]
})))
const questionFor = (item: Selection) => (mode.value === 'shared' ? sharedPrompt.value : item.prompt).trim()
const isAccountBusy = (id: number) => accountJobs.value.get(id)?.some(job => job.state === 'queued' || job.state === 'running') || false
const canRunAccount = (item: Selection) => !!questionFor(item) && !isAccountBusy(item.account.id) && !selectingAll.value && !historyLoading.value && !selectionSaving.value
function newJob(item: Selection): Job {
  return { account: { ...item.account }, prompt: questionFor(item), model: item.model.trim(), state: 'queued', result: null, error: '', startedAt: 0, endedAt: 0 }
}
function historyJob(record: AccountCheckRecord): Job {
  return { account: record.account, prompt: record.prompt, model: record.model, state: record.state,
    result: record.state === 'running' ? null : record.result, error: record.result.error || '',
    startedAt: Date.parse(record.started_at), endedAt: record.finished_at ? Date.parse(record.finished_at) : 0 }
}
const historyTime = (record: AccountCheckRecord) => new Date(record.finished_at || record.started_at).toLocaleString('zh-CN', { hour12: false })
function currentRecord(id: number) {
  return histories.value[id]?.find(record => record.id === viewedHistory.value[id]) || histories.value[id]?.[0]
}
async function loadHistory(id: number, restoreJob = true) {
  const records = await listAccountCheckHistory(id)
  if (disposed || !selectedIDs.value.has(id)) return
  histories.value[id] = records
  if (restoreJob && records.length && !runControllers.has(id)) {
    const job = historyJob(records[0]), index = jobs.value.findIndex(item => item.account.id === id)
    if (index < 0) jobs.value.push(job)
    else if (jobs.value[index].state !== 'queued') jobs.value.splice(index, 1, job)
  }
}
async function restoreHistory() {
  historyLoading.value = true; historyError.value = ''
  try {
    const [records, savedSelection] = await Promise.all([listAccountCheckHistory(), getAccountCheckSelection()])
    if (disposed) return
    const catalog: CheckAccount[] = []
    const firstPage = await listCheckAccounts(1, '')
    catalog.push(...firstPage.items)
    for (let current = 2; current <= Math.max(1, firstPage.pages || 1); current++) {
      catalog.push(...(await listCheckAccounts(current, '')).items)
    }
    if (disposed) return
    const catalogByID = new Map(catalog.map(account => [account.id, account]))
    const latestByID = new Map(records.map(record => [record.account.id, record]))
    const restoreIDs = savedSelection ?? records.map(record => record.account.id)
    for (const id of restoreIDs) {
      const account = catalogByID.get(id)
      if (!account || !addAccount(account)) continue
      const record = latestByID.get(id)
      const item = selections.value[selections.value.length - 1]
      if (record) { item.prompt = record.prompt; item.model = record.model }
    }
    const visibleRecords = records.filter(record => selectedIDs.value.has(record.account.id))
    if (visibleRecords.length) {
      const latest = [...visibleRecords].sort((a, b) => b.id - a.id)[0]
      if (!sharedPrompt.value) sharedPrompt.value = latest.prompt
      if (visibleRecords.some(record => record.prompt !== latest.prompt)) mode.value = 'individual'
    }
    const ids = selections.value.map(item => item.account.id)
    let next = 0
    await Promise.all(Array.from({ length: Math.min(4, ids.length) }, async () => {
      while (!disposed && next < ids.length) {
        const id = ids[next++]
        await loadHistory(id)
        await loadModels(id)
      }
    }))
  } catch (e) { if (!disposed) historyError.value = `历史记录加载失败：${message(e)}` }
  finally { if (!disposed) historyLoading.value = false }
}
async function pollHistory() {
  if (pollingHistory || historyLoading.value || disposed) return
  const ids = jobs.value.filter(job => job.state === 'running' && !runControllers.has(job.account.id)).map(job => job.account.id)
  pollingHistory = true
  try { for (const id of ids) await loadHistory(id) }
  catch (e) { if (!disposed) historyError.value = `检查进度恢复失败：${message(e)}` }
  finally { pollingHistory = false; processQueue() }
}
function groupLabel(account: CheckAccount) {
  const groups = new Map((account.groups || []).map(group => [group.id, group.name]))
  for (const id of account.group_ids || []) if (!groups.has(id)) groups.set(id, `分组 #${id}`)
  return groups.size ? [...groups].map(([id, name]) => `${name} (#${id})`).join('、') : '未绑定分组'
}
async function loadGroups() {
  groupsController?.abort()
  const controller = new AbortController(); groupsController = controller
  groupsLoading.value = true
  try {
    const data = await listCheckGroups(controller.signal)
    if (!controller.signal.aborted && !disposed) groups.value = data
  } catch (e) {
    if (!controller.signal.aborted && !disposed) listError.value = `分组加载失败：${message(e)}`
  } finally {
    if (!controller.signal.aborted && !disposed) groupsLoading.value = false
  }
}
function updateGroups(item: Selection, groupIDs: number[]) {
  const names = new Map(groups.value.map(group => [group.id, group.name]))
  item.account.group_ids = groupIDs
  item.account.groups = groupIDs.map(id => ({ id, name: names.get(id) || `分组 #${id}` }))
  const listed = accounts.value.find(account => account.id === item.account.id)
  if (listed) {
    listed.group_ids = [...groupIDs]
    listed.groups = item.account.groups.map(group => ({ ...group }))
  }
}
async function loadAccounts(targetPage = 1) {
  listController?.abort()
  const controller = new AbortController(); listController = controller
  loading.value = true; listError.value = ''
  try {
    const data = await listCheckAccounts(targetPage, search.value.trim(), controller.signal)
    if (controller.signal.aborted || disposed) return
    accounts.value = data.items; total.value = data.total; page.value = targetPage; pages.value = Math.max(1, data.pages || Math.ceil(data.total / 50))
  } catch (e) { if (!controller.signal.aborted && !disposed) listError.value = message(e) }
  finally { if (!controller.signal.aborted && !disposed) loading.value = false }
}
function searchAccounts() {
  listController?.abort(); clearTimeout(searchTimer)
  searchTimer = setTimeout(() => void loadAccounts(), 300)
}
function addAccount(account: CheckAccount) {
  if (selectedIDs.value.has(account.id)) return false
  selections.value.push({ account, prompt: '', model: DEFAULT_CHECK_MODEL, models: [], loadingModels: true, modelError: '' })
  return true
}
async function persistSelection(ids: number[]) {
  selectionSaving.value = true
  try { await saveAccountCheckSelection(ids); return !disposed }
  catch (e) { if (!disposed) historyError.value = `账号选择保存失败，未更改选择：${message(e)}`; return false }
  finally { if (!disposed) selectionSaving.value = false }
}
async function toggleAccount(account: CheckAccount) {
  if (running.value || selectingAll.value || selectionSaving.value || historyLoading.value) return
  if (selectedIDs.value.has(account.id)) {
    if (!await persistSelection(selections.value.filter(item => item.account.id !== account.id).map(item => item.account.id))) return
    modelControllers.get(account.id)?.abort(); modelControllers.delete(account.id)
    selections.value = selections.value.filter(item => item.account.id !== account.id)
    jobs.value = jobs.value.filter(job => job.account.id !== account.id)
    return
  }
  if (!await persistSelection([...selectedIDs.value, account.id])) return
  if (addAccount(account)) {
    void loadModels(account.id)
    void loadHistory(account.id).catch(e => { historyError.value = message(e) })
  }
}
async function loadModels(id: number) {
  const item = selections.value.find(selection => selection.account.id === id)
  if (!item) return
  const controller = new AbortController(); modelControllers.set(id, controller)
  try {
    const models = await listCheckModels(id, controller.signal)
    if (!controller.signal.aborted && !disposed) item.models = models
  } catch (e) { if (!controller.signal.aborted && !disposed) item.modelError = `${message(e)}；可留空使用默认模型，或手动输入。` }
  finally { if (!controller.signal.aborted && !disposed) item.loadingModels = false }
}
async function loadModelsInBatches(ids: number[]) {
  let next = 0
  const worker = async () => {
    while (!disposed && next < ids.length) await loadModels(ids[next++])
  }
  await Promise.all(Array.from({ length: Math.min(4, ids.length) }, worker))
}
async function selectAllAccounts() {
  if (running.value || selectingAll.value || selectionSaving.value || historyLoading.value || total.value < 1) return
  selectAllController?.abort()
  const controller = new AbortController(); selectAllController = controller
  selectingAll.value = true; listError.value = ''
  const query = search.value.trim()
  try {
    const first = await listCheckAccounts(1, query, controller.signal)
    const all = [...first.items]
    const pageCount = Math.max(1, first.pages || Math.ceil(first.total / 50))
    for (let current = 2; current <= pageCount; current++) {
      const data = await listCheckAccounts(current, query, controller.signal)
      all.push(...data.items)
    }
    if (controller.signal.aborted || disposed) return
    if (!await persistSelection([...new Set([...selectedIDs.value, ...all.map(account => account.id)])])) return
    const modelIDs: number[] = []
    for (const account of all) if (addAccount(account)) modelIDs.push(account.id)
    void loadModelsInBatches(modelIDs)
    for (const id of modelIDs) void loadHistory(id).catch(e => { if (!disposed) historyError.value = message(e) })
  } catch (e) {
    if (!controller.signal.aborted && !disposed) listError.value = `全选失败：${message(e)}`
  } finally {
    if (!controller.signal.aborted && !disposed) selectingAll.value = false
  }
}
async function clearSelection() {
  if (running.value || selectingAll.value || selectionSaving.value || historyLoading.value) return
  if (!await persistSelection([])) return
  modelControllers.forEach(controller => controller.abort()); modelControllers.clear(); selections.value = []; jobs.value = []
}
function copySharedPrompt() {
  if (!running.value) for (const item of selections.value) item.prompt = sharedPrompt.value
}
async function cancelJob(job: Job) {
  if (job.state === 'queued') { job.state = 'cancelled'; job.error = '已取消，未发起检查' }
  if (job.state === 'running') {
    if (cancellingIDs.value.has(job.account.id)) return
    cancellingIDs.value = new Set(cancellingIDs.value).add(job.account.id)
    try {
      const result = await cancelAccountCheck(job.account.id)
      if (!result.cancelled) throw new Error('本次检查已结束，请刷新记录')
      runControllers.get(job.account.id)?.abort('cancelled')
      job.state = 'cancelled'; job.error = '检查已由管理员终止'; job.endedAt = Date.now()
      await loadHistory(job.account.id)
    } catch (e) {
      if (!disposed) historyError.value = `终止检查失败：${message(e)}`
    } finally {
      const next = new Set(cancellingIDs.value); next.delete(job.account.id); cancellingIDs.value = next
    }
  }
}
function cancelAll() { for (const job of jobs.value) void cancelJob(job) }
async function execute(job: Job) {
  const controller = new AbortController(); runControllers.set(job.account.id, controller)
  job.state = 'running'; job.startedAt = Date.now()
  const timeout = setTimeout(() => controller.abort('timeout'), 6 * 60_000)
  try {
    const result = await runAccountCheck(job.account.id, { prompt: job.prompt, model_id: job.model }, controller.signal)
    if (disposed) return
    if (controller.signal.aborted) throw new DOMException('Aborted', 'AbortError')
    job.result = result; job.error = result.error || ''; job.state = result.error ? 'failed' : 'succeeded'
  } catch (e) {
    if (disposed) return
    job.state = controller.signal.aborted && controller.signal.reason !== 'timeout' ? 'cancelled' : 'failed'
    job.error = controller.signal.aborted ? (controller.signal.reason === 'timeout' ? '检查等待超时，请重试' : '检查已由管理员终止') : message(e)
  } finally {
    clearTimeout(timeout); runControllers.delete(job.account.id); job.endedAt = Date.now()
    if (!disposed) {
      try { await loadHistory(job.account.id) }
      catch (e) { historyError.value = `记录同步失败：${message(e)}` }
    }
  }
}
function processQueue() {
  if (disposed) return
  while (activeWorkers < 3 && jobs.value.filter(job => job.state === 'running').length < 3) {
    const job = jobs.value.find(item => item.state === 'queued')
    if (!job) break
    activeWorkers++
    if (!timer) {
      now.value = Date.now()
      timer = setInterval(() => { now.value = Date.now() }, 1000)
    }
    void execute(job).finally(() => { activeWorkers--; processQueue() })
  }
}
function run() {
  if (!canRun.value) return
  jobs.value = selections.value.map(newJob)
  viewedHistory.value = {}
  processQueue()
}
function runAccount(item: Selection) {
  if (!canRunAccount(item)) return
  delete viewedHistory.value[item.account.id]
  const index = jobs.value.findIndex(job => job.account.id === item.account.id)
  if (index < 0) jobs.value.push(newJob(item))
  else jobs.value.splice(index, 1, newJob(item))
  processQueue()
}
onMounted(() => {
  void loadAccounts(); void loadGroups(); void restoreHistory()
  timer = setInterval(() => { now.value = Date.now() }, 1000)
  historyPoll = setInterval(() => { void pollHistory() }, 5000)
})
onUnmounted(() => { disposed = true; listController?.abort(); selectAllController?.abort(); groupsController?.abort(); modelControllers.forEach(controller => controller.abort()); runControllers.forEach(controller => controller.abort()); clearInterval(timer); clearInterval(historyPoll); clearTimeout(searchTimer) })
</script>

<template>
  <AppLayout viewport>
    <div class="check-workbench">
      <header class="workbench-heading">
        <div><div class="eyebrow"><i></i> OPENAI · 账号检测</div><h1>账号降智检查</h1><p>选择账号、设置题目，并排对比回答与动画效果。</p></div>
        <div class="heading-count"><strong>{{ selections.length }}</strong>已选账号<span></span><strong>3</strong>最大并发</div>
      </header>
      <form class="check-console" @submit.prevent="run">
        <aside class="account-picker" aria-label="选择检查账号">
          <div class="panel-heading"><h2><b class="step">01</b>选择账号</h2><span class="count-tag">{{ total }} 个</span></div>
          <div class="picker-controls">
            <input v-model="search" aria-label="搜索账号" class="input w-full" placeholder="搜索账号名称…" :disabled="running || selectingAll" @input="searchAccounts" />
            <div class="picker-actions"><button type="button" data-select-all class="select-all-button" :disabled="loading || running || selectingAll || selectionSaving || historyLoading || !total" @click="selectAllAccounts"><Icon name="check" size="sm" />{{ selectingAll ? '正在全选…' : '全选账号' }}<span>{{ total }}</span></button><button type="button" class="text-action muted" :disabled="running || selectingAll || selectionSaving || historyLoading || !selections.length" @click="clearSelection">取消全选</button><span>{{ selectionSaving ? '保存中…' : `已选 ${selections.length}` }}</span></div>
          </div>
          <div class="picker-list">
            <p v-if="loading" role="status" class="small-empty">正在加载账号…</p>
            <template v-else>
              <label v-for="account in accounts" :key="account.id" class="picker-row" :class="{ selected: selectedIDs.has(account.id) }">
                <input type="checkbox" :data-account-id="account.id" :checked="selectedIDs.has(account.id)" :disabled="running || selectingAll || selectionSaving || historyLoading" @change="toggleAccount(account)" />
                <span class="picker-identity"><strong>#{{ account.id }} · {{ account.name }}</strong><span class="picker-groups" :title="groupLabel(account)">{{ groupLabel(account) }}</span></span>
              </label>
              <p v-if="!accounts.length" class="small-empty">暂无匹配账号</p>
            </template>
          </div>
          <div class="picker-pagination"><span>{{ page }} / {{ pages }} 页</span><div><button type="button" class="btn btn-secondary btn-sm" :disabled="loading || running || selectingAll || page <= 1" @click="loadAccounts(page - 1)">上一页</button><button type="button" class="btn btn-secondary btn-sm" :disabled="loading || running || selectingAll || page >= pages" @click="loadAccounts(page + 1)">下一页</button></div></div>
          <p v-if="listError" role="alert" class="picker-error">{{ listError }} <button type="button" class="underline" @click="loadAccounts(); loadGroups()">重新加载</button></p>
        </aside>
        <div class="check-configuration">
          <section class="question-panel" aria-label="设置检查题目">
            <div class="panel-heading"><h2><b class="step">02</b>设置题目</h2><fieldset :disabled="running" class="question-switch"><legend class="sr-only">出题方式</legend><label :class="{ active: mode === 'shared' }"><input v-model="mode" type="radio" name="question-mode" value="shared" /><span>统一题目</span></label><label :class="{ active: mode === 'individual' }"><input v-model="mode" type="radio" name="question-mode" value="individual" /><span>逐账号出题</span></label></fieldset></div>
            <div v-show="mode === 'shared'" class="prompt-editor"><label for="check-prompt" class="sr-only">统一检查题目</label><textarea id="check-prompt" v-model="sharedPrompt" class="input" rows="4" maxlength="16000" :disabled="running" placeholder="输入推理题、写作要求，或要求生成完整 HTML 页面 / SVG 动画…" /><div class="prompt-meta"><span>所有已选账号分别回答同一道题</span><span>{{ sharedPrompt.length.toLocaleString() }} / 16,000</span></div></div>
            <div v-if="mode === 'individual'" class="individual-hint"><p>在下方账号卡片中分别填写题目。</p><button v-if="sharedPrompt.trim()" type="button" class="text-action" :disabled="running" @click="copySharedPrompt">用统一题目填充全部</button></div>
          </section>
          <section class="selection-panel" aria-label="已选账号配置">
            <p v-if="historyLoading" class="history-notice" role="status">正在恢复已保存的检查记录…</p>
            <p v-if="historyError" class="history-notice history-error" role="alert">{{ historyError }} <button type="button" class="underline" :disabled="running || historyLoading" @click="restoreHistory">重试</button></p>
            <div class="selection-heading"><h2>账号与结果 <span class="count-tag">{{ selections.length }}</span></h2><span v-if="jobs.length" class="batch-summary"><span class="success-count">成功 {{ succeeded }}</span> · 失败 {{ failed }} · 已结束 {{ completed }} / {{ jobs.length }}</span><span v-else>每个账号独立配置，结果显示在卡片内</span></div>
            <div v-if="!selections.length" class="selection-empty"><Icon name="plus" size="md" /><div><strong>从左侧选择账号</strong><p>支持多选，各账号可独立设置模型和题目。</p></div></div>
            <div v-else class="selected-scroll"><div class="selected-grid">
              <article v-for="item in selections" :key="item.account.id" :data-selected-account="item.account.id" class="selected-card">
                <div class="selected-card-heading"><h3>#{{ item.account.id }} · {{ item.account.name }}</h3><button type="button" class="text-action muted" :disabled="running || selectingAll || selectionSaving || historyLoading" :aria-label="`移除账号 ${item.account.id}`" @click="toggleAccount(item.account)">移除</button></div>
                <details class="selected-groups"><summary :title="groupLabel(item.account)">绑定分组：{{ groupLabel(item.account) }}</summary><p>{{ groupLabel(item.account) }}</p></details>
                <label class="model-field"><span>检查模型</span><input v-model="item.model" :aria-label="`账号 ${item.account.id} 的模型`" :list="`check-models-${item.account.id}`" class="input" :disabled="isAccountBusy(item.account.id)" :placeholder="item.loadingModels ? '正在加载可用模型…' : DEFAULT_CHECK_MODEL" /></label>
                <datalist :id="`check-models-${item.account.id}`"><option v-for="model in item.models" :key="model.id" :value="model.id">{{ model.display_name }}</option></datalist>
                <p v-if="item.modelError" role="alert" class="model-error">{{ item.modelError }}</p>
                <label v-if="mode === 'individual'" class="individual-question"><span>该账号的检查题目</span><textarea v-model="item.prompt" :aria-label="`账号 ${item.account.id} 的题目`" class="input" rows="3" maxlength="16000" :disabled="isAccountBusy(item.account.id)" placeholder="输入仅发送给这个账号的题目" /></label>
                <AccountCheckManagement :account="item.account" :groups="groups" :groups-loading="groupsLoading" :models="item.models" :disabled="isAccountBusy(item.account.id)" @groups-updated="updateGroups(item, $event)">
                  <template #actions><button type="button" class="btn btn-primary btn-sm card-run-button" :data-run-account="item.account.id" :aria-label="`检查账号 ${item.account.id}`" :disabled="!canRunAccount(item)" :title="questionFor(item) ? '仅检查此账号' : mode === 'shared' ? '请先填写统一题目' : '请先填写该账号的题目'" @click="runAccount(item)"><Icon :name="isAccountBusy(item.account.id) ? 'refresh' : 'play'" size="xs" :class="{ 'animate-spin': isAccountBusy(item.account.id) }" />{{ isAccountBusy(item.account.id) ? '检查中…' : '开始检查' }}</button></template>
                </AccountCheckManagement>
                <div v-if="histories[item.account.id]?.length" class="card-history">
                  <div><span>检查记录</span><time v-if="currentRecord(item.account.id)">{{ historyTime(currentRecord(item.account.id)!) }}</time></div>
                  <label><span>最近 {{ histories[item.account.id].length }} 次</span><select :value="viewedHistory[item.account.id] || histories[item.account.id][0].id" :disabled="isAccountBusy(item.account.id)" :aria-label="`账号 ${item.account.id} 的历史记录`" @change="viewedHistory[item.account.id] = Number(($event.target as HTMLSelectElement).value)"><option v-for="(record, index) in histories[item.account.id]" :key="record.id" :value="record.id">{{ index === 0 ? '最新 · ' : '' }}{{ historyTime(record) }} · {{ record.state === 'succeeded' ? '成功' : record.state === 'running' ? '检查中' : record.state === 'cancelled' ? '已终止' : '失败' }}</option></select></label>
                </div>
                <AccountCheckResult v-for="job in accountJobs.get(item.account.id)" :key="job.account.id" embedded :account="job.account" :prompt="job.prompt" :model="job.model" :state="job.state" :result="job.result" :error="job.error" :elapsed="job.startedAt ? Math.max(0, Math.floor(((job.endedAt || now) - job.startedAt) / 1000)) : 0" :cancelling="cancellingIDs.has(job.account.id)" @cancel="cancelJob(job)" />
                <div v-if="!accountJobs.has(item.account.id)" class="card-result-placeholder"><Icon name="play" size="sm" /><span>检查后在这里显示答案或动画</span></div>
              </article>
            </div></div>
          </section>
          <div class="run-toolbar"><div class="run-context"><i></i><span v-if="running" role="status">已完成 {{ completed }} / {{ jobs.length }} · 最多同时检查 3 个账号</span><span v-else-if="!selections.length">选择账号后开始检查</span><span v-else-if="!validQuestions">{{ mode === 'shared' ? '请输入统一题目' : '请为每个已选账号填写题目' }}</span><span v-else>已就绪 · {{ selections.length }} 个账号等待检查</span></div><div class="run-buttons"><button v-if="running" type="button" class="btn btn-secondary" @click="cancelAll">停止全部</button><button type="submit" class="btn btn-primary" :disabled="!canRun"><Icon :name="running ? 'refresh' : 'play'" size="sm" :class="{ 'animate-spin': running }" />{{ running ? '检查中…' : `开始检查${selections.length ? `（${selections.length} 个账号）` : ''}` }}</button></div></div>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<style scoped>
.check-workbench { --line: #e2e9e8; --surface: #fff; --subtle: #f7faf9; --ink: #1c3431; --muted: #667e77; --accent: #0d9488; height: 100%; min-height: 0; display: flex; flex-direction: column; margin: 0 auto; color: var(--ink); }
.workbench-heading { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 20px; margin-bottom: 12px; }
.eyebrow { display: flex; align-items: center; gap: 7px; font-size: 10px; font-weight: 700; letter-spacing: .12em; color: var(--accent); }
.eyebrow i, .run-context i { display: inline-block; flex-shrink: 0; width: 6px; height: 6px; border-radius: 50%; background: var(--accent); }
.workbench-heading h1 { margin: 2px 0; font-size: 21px; font-weight: 700; letter-spacing: -.03em; }
.workbench-heading p { color: var(--muted); font-size: 12px; }
.heading-count { display: flex; align-items: center; gap: 9px; font-size: 11px; color: var(--muted); }
.heading-count strong { font-size: 24px; color: var(--ink); font-weight: 600; font-variant-numeric: tabular-nums; }
.heading-count > span { width: 1px; height: 25px; background: var(--line); margin: 0 12px; }
.check-console { flex: 1; min-height: 0; display: grid; grid-template-columns: 280px minmax(0, 1fr); grid-template-rows: minmax(0, 1fr); overflow: hidden; border: 1px solid var(--line); border-radius: 14px; background: var(--surface); box-shadow: 0 4px 20px #163e3104; }
.panel-heading { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px; padding: 10px 14px; }
.panel-heading h2 { display: flex; align-items: center; gap: 9px; font-size: 14px; font-weight: 600; }
.step { display: inline-grid; place-items: center; width: 23px; height: 23px; border: 1px solid var(--line); border-radius: 7px; color: var(--muted); font-size: 10px; font-weight: 600; }
.count-tag { display: inline-block; padding: 2px 7px; border-radius: 5px; background: var(--subtle); color: var(--muted); font-size: 11px; font-weight: 500; }
.account-picker { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; border-right: 1px solid var(--line); background: var(--subtle); }
.picker-controls { flex-shrink: 0; padding: 0 12px 10px; }
.check-workbench .input { min-width: 0; font-size: 12px; border-radius: 8px; }
.picker-controls .input { background: var(--surface); }
.picker-actions { display: flex; align-items: center; gap: 8px; margin-top: 9px; font-size: 11px; }
.picker-actions > span { margin-left: auto; color: var(--muted); }
.select-all-button { display: inline-flex; align-items: center; justify-content: center; gap: 5px; padding: 7px 9px; border: 1px solid var(--accent); border-radius: 7px; background: var(--accent); color: #fff; font-size: 12px; font-weight: 600; white-space: nowrap; }
.select-all-button > span { padding: 0 4px; border-radius: 4px; background: #ffffff25; font-size: 11px; }
.select-all-button:hover:not(:disabled) { filter: brightness(.93); }
.select-all-button:disabled { opacity: .45; cursor: not-allowed; }
.text-action { color: var(--accent); font-size: 11px; font-weight: 500; white-space: nowrap; }
.text-action:hover:not(:disabled) { text-decoration: underline; }
.text-action:disabled { opacity: .4; cursor: not-allowed; }
.text-action.muted { color: var(--muted); }
.picker-list { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 0 8px 8px; scrollbar-width: thin; }
.picker-row { display: flex; align-items: flex-start; gap: 9px; padding: 10px 9px; border: 1px solid transparent; border-radius: 8px; cursor: pointer; transition: background .15s; }
.picker-row + .picker-row { margin-top: 3px; }
.picker-row:hover { background: var(--surface); }
.picker-row.selected { background: #ecf8f4; border-color: #c8e8df; }
.picker-row input { flex-shrink: 0; margin-top: 3px; accent-color: var(--accent); }
.picker-identity { min-width: 0; flex: 1; font-size: 12px; overflow-wrap: anywhere; }
.picker-identity strong { font-weight: 600; }
.picker-groups { display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 2; overflow: hidden; margin-top: 4px; font-size: 11px; line-height: 1.6; color: var(--muted); }
.picker-pagination { display: flex; flex-shrink: 0; justify-content: space-between; align-items: center; padding: 8px 12px; border-top: 1px solid var(--line); color: var(--muted); font-size: 11px; }
.picker-pagination > div { display: flex; gap: 5px; }
.picker-error { padding: 10px 14px; font-size: 12px; color: #dc2626; }
.check-configuration { display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; }
.question-panel { flex-shrink: 0; }
.question-switch { display: flex; padding: 3px; background: var(--subtle); border: 1px solid var(--line); border-radius: 8px; }
.question-switch label { position: relative; border-radius: 5px; padding: 5px 10px; cursor: pointer; color: var(--muted); font-size: 12px; }
.question-switch label.active { color: var(--accent); background: var(--surface); box-shadow: 0 1px 4px #163e3110; font-weight: 600; }
.question-switch input { position: absolute; opacity: 0; width: 1px; height: 1px; }
.question-switch label:focus-within { outline: 2px solid var(--accent); outline-offset: 2px; }
.prompt-editor { margin: 0 14px 10px; }
.prompt-editor textarea { width: 100%; display: block; height: clamp(56px, 8dvh, 76px); min-height: 0; resize: none; line-height: 1.8; background: var(--subtle); }
.prompt-meta { display: flex; justify-content: space-between; gap: 12px; margin-top: 7px; color: var(--muted); font-size: 11px; }
.individual-hint { display: flex; justify-content: space-between; gap: 10px; padding: 0 18px 15px; font-size: 12px; color: var(--muted); }
.selection-panel { display: flex; flex-direction: column; border-top: 1px solid var(--line); flex: 1; min-width: 0; min-height: 0; overflow: hidden; }
.selection-heading { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 10px; padding: 9px 14px; }
.selection-heading h2 { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 600; }
.selection-heading > span { color: var(--muted); font-size: 11px; }
.selected-scroll { flex: 1; min-height: 0; overflow-y: auto; overscroll-behavior: contain; padding: 0 14px 10px; scrollbar-width: thin; }
.selected-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 260px), 1fr)); gap: 12px; align-items: start; }
.selected-card { container-type: inline-size; min-width: 0; padding: 12px; border: 1px solid var(--line); border-radius: 10px; background: var(--surface); box-shadow: 0 2px 5px #163e3105; }
.card-result-placeholder { display: flex; align-items: center; justify-content: center; gap: 7px; margin-top: 10px; height: 64px; border: 1px dashed var(--line); border-radius: 7px; background: var(--subtle); font-size: 11px; color: var(--muted); }
.card-run-button { font-size: 10px; padding: 4px 5px; gap: 4px; border-radius: 6px; min-height: 27px; }
.history-notice { flex-shrink: 0; padding: 8px 14px; font-size: 12px; color: var(--muted); }
.history-error { color: #b45309; }
.card-history { margin-top: 10px; padding-top: 8px; border-top: 1px solid var(--line); font-size: 10px; color: var(--muted); }
.card-history > div { display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px; }
.card-history label { display: flex; align-items: center; gap: 6px; margin-top: 6px; }
.card-history label > span { flex-shrink: 0; }
.card-history select { min-width: 0; width: 100%; padding: 4px; border: 1px solid var(--line); border-radius: 5px; background: var(--subtle); font-size: 10px; }
.selected-card-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
.selected-card h3 { font-size: 12px; font-weight: 600; min-width: 0; overflow-wrap: anywhere; }
.selected-groups { margin: 5px 0 11px; font-size: 11px; color: var(--muted); }
.selected-groups summary { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; cursor: pointer; }
.selected-groups p { padding-top: 5px; overflow-wrap: anywhere; line-height: 1.7; }
.model-field { display: grid; grid-template-columns: auto minmax(0, 1fr); align-items: center; gap: 12px; font-size: 11px; color: var(--muted); }
.model-field .input { width: 100%; height: 33px; padding: 6px 9px; background: var(--subtle); }
.model-error { margin-top: 6px; color: #b45309; font-size: 11px; }
.individual-question { display: block; margin-top: 10px; color: var(--muted); font-size: 11px; }
.individual-question textarea { display: block; width: 100%; resize: vertical; margin-top: 5px; line-height: 1.7; }
.selection-empty { display: flex; flex: 1; min-height: 0; overflow: auto; align-items: center; justify-content: center; gap: 12px; color: var(--muted); font-size: 12px; padding: 14px; }
.selection-empty strong { font-weight: 500; color: var(--ink); }
.selection-empty p { margin-top: 5px; font-size: 11px; }
.run-toolbar { display: flex; flex-shrink: 0; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap; padding: 10px 14px; background: var(--subtle); border-top: 1px solid var(--line); }
.run-context, .run-buttons { display: flex; align-items: center; gap: 10px; }
.run-context { color: var(--muted); font-size: 11px; }
.run-buttons { margin-left: auto; }
.run-buttons .btn { font-size: 12px; }
.success-count { color: var(--accent); }
.small-empty { padding: 32px 10px; text-align: center; font-size: 12px; color: var(--muted); }
:global(.dark) .check-workbench { --line: #2a3a3c; --surface: #162224; --subtle: #111d1f; --ink: #e0eeea; --muted: #8baba2; --accent: #4dd4bb; }
:global(.dark) .picker-row.selected { background: #173c36; border-color: #276052; }
@media (min-width: 1600px) { .check-console { grid-template-columns: 300px minmax(0, 1fr); } }
@media (max-width: 1000px) { .check-console { grid-template-columns: 250px minmax(0, 1fr); } .heading-count, .run-context { display: none; } }
@media (max-width: 700px) {
  .check-console { grid-template-columns: minmax(0, 1fr); grid-template-rows: minmax(0, .7fr) minmax(0, 1fr); }
  .account-picker { border-right: 0; border-bottom: 1px solid var(--line); }
  .account-picker .panel-heading { padding: 7px 12px; }
  .picker-controls { display: flex; gap: 8px; align-items: center; padding-bottom: 7px; }
  .picker-controls > input { flex: 1; width: 0; }
  .picker-actions { flex-shrink: 0; margin-top: 0; }
  .picker-actions > span { display: none; }
  .picker-pagination { padding: 5px 12px; }
  .workbench-heading h1 { font-size: 18px; }
  .workbench-heading { margin-bottom: 8px; }
  .eyebrow, .workbench-heading p, .selection-heading > span:not(.batch-summary), .prompt-meta { display: none; }
  .check-configuration { overflow-y: auto; overscroll-behavior: contain; }
  .question-panel .panel-heading { padding: 7px 12px; }
  .selection-panel { flex: none; min-height: 110px; max-height: 220px; }
  .run-toolbar { position: sticky; bottom: 0; z-index: 1; margin-top: auto; }
}
@media (max-height: 720px) and (min-width: 701px) {
  .eyebrow, .workbench-heading p, .prompt-meta { display: none; }
  .workbench-heading { margin-bottom: 8px; }
  .question-panel .panel-heading { padding: 7px 14px; }
  .prompt-editor textarea { height: 56px; }
}
</style>
