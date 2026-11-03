<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import AccountCheckResult from './AccountCheckResult.vue'
import { defaultCheckSchedule, parseCheckSchedule, scheduleCron, scheduleDescription, weekdays } from './checkSchedule'
import {
  createScheduledCheck,
  deleteScheduledCheck,
  listScheduledCheckResults,
  listScheduledChecks,
  listLegacyScheduledChecks,
  pauseLegacyScheduledCheck,
  updateCheckAccountGroups,
  updateScheduledCheck,
  type CheckAccount,
  type CheckGroupOption,
  type ScheduledCheckPlan,
  type ScheduledCheckResult
} from '@/api/admin/accountChecks'
import type { AccountCheckResult as CheckResult } from '@/api/admin/accountChecks'

const props = defineProps<{
  account: CheckAccount
  groups: CheckGroupOption[]
  groupsLoading: boolean
  models: Array<{ id: string; display_name: string }>
  selectedModel?: string
  prompt?: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  (event: 'groups-updated', groupIds: number[]): void
}>()

const groupEditorOpen = ref(false)
const selectedGroupIDs = ref<number[]>([])
const savingGroups = ref(false)
const groupError = ref('')
const scheduleOpen = ref(false)
const scheduleLoading = ref(false)
const scheduleError = ref('')
const plans = ref<ScheduledCheckPlan[]>([])
const legacyPlans = ref<ScheduledCheckPlan[]>([])
const legacyError = ref('')
const latestResults = reactive<Record<number, ScheduledCheckResult | null>>({})
const displayResults = reactive<Record<number, CheckResult>>({})
const histories = reactive<Record<number, ScheduledCheckResult[]>>({})
const historyOpen = reactive<Record<number, boolean>>({})
const historyLoading = reactive<Record<number, boolean>>({})
const editingPlanID = ref<number | null>(null)
const savingPlan = ref(false)
const deletingPlanID = ref<number | null>(null)
const newPlanOpen = ref(false)
const form = reactive({ model_id: '', reasoning_effort: 'high', prompt: '', enabled: true })
const schedule = reactive(defaultCheckSchedule())
const cronExpression = computed(() => scheduleCron(schedule))
const modelOptions = computed(() => {
  const options = new Map(props.models.map(model => [model.id, model.display_name]))
  for (const id of [props.selectedModel, form.model_id]) if (id && !options.has(id)) options.set(id, id)
  return Array.from(options, ([id, name]) => ({ id, name }))
})
const effortOptions = [
  { value: '', label: '模型默认' }, { value: 'none', label: '不思考（none）' },
  { value: 'minimal', label: '最少（minimal）' }, { value: 'low', label: '低（low）' },
  { value: 'medium', label: '中（medium）' }, { value: 'high', label: '高（high）' },
  { value: 'xhigh', label: '超高（xhigh）' }
]
const effortLabel = (value?: string) => effortOptions.find(option => option.value === (value || ''))?.label || value

const errorMessage = (error: unknown) => (error as { message?: string })?.message || '操作失败，请重试'
const formatTime = (value: string | null | undefined) => {
  if (!value) return '尚未执行'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { timeZone: 'Asia/Shanghai', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date)
}
const statusText = (status: string) => status === 'success' ? '成功' : status === 'running' ? '执行中' : '失败'
const statusClass = (status: string) => status === 'success' ? 'text-green-700 dark:text-green-400' : status === 'running' ? 'text-blue-700 dark:text-blue-400' : 'text-red-700 dark:text-red-400'

function openGroupEditor() {
  selectedGroupIDs.value = [...(props.account.group_ids || [])]
  groupError.value = ''
  groupEditorOpen.value = true
}

async function saveGroups() {
  if (savingGroups.value) return
  savingGroups.value = true
  groupError.value = ''
  try {
    await updateCheckAccountGroups(props.account.id, selectedGroupIDs.value)
    emit('groups-updated', [...selectedGroupIDs.value])
    groupEditorOpen.value = false
  } catch (error) {
    groupError.value = errorMessage(error)
  } finally {
    savingGroups.value = false
  }
}

async function loadLatest(plan: ScheduledCheckPlan) {
  try {
    const results = await listScheduledCheckResults(props.account.id, plan.id, 1)
    const latest = results[0] || null
    const previous = latestResults[plan.id]
    if (latest?.id !== previous?.id || latest?.status !== previous?.status || latest?.finished_at !== previous?.finished_at) {
      latestResults[plan.id] = latest
      if (latest) displayResults[plan.id] = { text: latest.response_text, html: latest.html || '', model: plan.model_id, error: latest.error_message }
    }
  } catch (error) {
    scheduleError.value = errorMessage(error)
    latestResults[plan.id] = null
  }
}

async function loadPlans() {
  if (scheduleLoading.value) return
  scheduleLoading.value = true
  scheduleError.value = ''
  try {
    plans.value = await listScheduledChecks(props.account.id)
    await Promise.all(plans.value.map(loadLatest))
    try {
      legacyPlans.value = await listLegacyScheduledChecks(props.account.id)
      legacyError.value = ''
    } catch {
      legacyError.value = '旧版计划读取失败，请刷新确认是否有仍在运行的旧计划。'
    }
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    scheduleLoading.value = false
  }
}

async function toggleSchedule() {
  scheduleOpen.value = !scheduleOpen.value
  if (scheduleOpen.value && !plans.value.length) {
    await loadPlans()
    if (!plans.value.length && !scheduleError.value) beginCreate()
  }
}

function beginCreate() {
  editingPlanID.value = null
  form.model_id = props.selectedModel?.trim() || props.models[0]?.id || 'gpt-6-astra'
  form.reasoning_effort = 'high'
  form.prompt = props.prompt?.trim() || 'hi'
  Object.assign(schedule, defaultCheckSchedule())
  form.enabled = true
  newPlanOpen.value = true
}

function beginEdit(plan: ScheduledCheckPlan) {
  editingPlanID.value = plan.id
  form.model_id = plan.model_id || 'gpt-6-astra'
  form.reasoning_effort = plan.reasoning_effort || ''
  form.prompt = plan.prompt || 'hi'
  Object.assign(schedule, parseCheckSchedule(plan.cron_expression))
  form.enabled = plan.enabled
  newPlanOpen.value = false
}

function cancelPlanForm() {
  editingPlanID.value = null
  newPlanOpen.value = false
}

async function savePlan() {
  if (savingPlan.value || !form.model_id.trim() || !form.prompt.trim() || !cronExpression.value) return
  savingPlan.value = true
  scheduleError.value = ''
  try {
    const payload = { ...form, model_id: form.model_id.trim(), cron_expression: cronExpression.value }
    if (editingPlanID.value) {
      const updated = await updateScheduledCheck(props.account.id, editingPlanID.value, payload)
      const index = plans.value.findIndex(plan => plan.id === updated.id)
      if (index >= 0) plans.value[index] = updated
    } else {
      const created = await createScheduledCheck(props.account.id, payload)
      plans.value.unshift(created)
      latestResults[created.id] = null
    }
    cancelPlanForm()
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    savingPlan.value = false
  }
}

async function toggleEnabled(plan: ScheduledCheckPlan) {
  if (savingPlan.value) return
  savingPlan.value = true
  scheduleError.value = ''
  try {
    const updated = await updateScheduledCheck(props.account.id, plan.id, { enabled: !plan.enabled })
    const index = plans.value.findIndex(item => item.id === plan.id)
    if (index >= 0) plans.value[index] = updated
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    savingPlan.value = false
  }
}

async function removePlan(plan: ScheduledCheckPlan) {
  if (!window.confirm(`确认删除计划 #${plan.id}？`)) return
  deletingPlanID.value = plan.id
  scheduleError.value = ''
  try {
    await deleteScheduledCheck(props.account.id, plan.id)
    plans.value = plans.value.filter(item => item.id !== plan.id)
    delete latestResults[plan.id]
    delete displayResults[plan.id]
    delete histories[plan.id]
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    deletingPlanID.value = null
  }
}

async function toggleHistory(plan: ScheduledCheckPlan) {
  historyOpen[plan.id] = !historyOpen[plan.id]
  if (!historyOpen[plan.id]) return
  historyLoading[plan.id] = true
  scheduleError.value = ''
  try {
    histories[plan.id] = await listScheduledCheckResults(props.account.id, plan.id, 3)
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    historyLoading[plan.id] = false
  }
}

async function pauseLegacy(plan: ScheduledCheckPlan) {
  if (savingPlan.value) return
  savingPlan.value = true
  try {
    const updated = await pauseLegacyScheduledCheck(props.account.id, plan.id)
    const index = legacyPlans.value.findIndex(item => item.id === plan.id)
    if (index >= 0) legacyPlans.value[index] = updated
  } catch (error) { scheduleError.value = errorMessage(error) }
  finally { savingPlan.value = false }
}

const schedulePoll = setInterval(() => {
  if (scheduleOpen.value && !savingPlan.value && !scheduleLoading.value) void loadPlans()
}, 10000)
onUnmounted(() => clearInterval(schedulePoll))
</script>

<template>
  <div class="check-management space-y-3">
    <div class="management-actions flex flex-wrap gap-2">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="disabled" @click="groupEditorOpen ? groupEditorOpen = false : openGroupEditor()"><Icon name="edit" size="xs" />修改分组</button>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="disabled" @click="toggleSchedule"><Icon name="calendar" size="xs" />定时检查</button>
      <slot name="actions" />
    </div>

    <div v-if="groupEditorOpen" class="rounded border border-gray-200 bg-gray-50 p-3 dark:border-dark-500 dark:bg-dark-700/40">
      <p class="mb-2 text-xs font-medium">选择绑定分组</p>
      <p v-if="groupsLoading" class="text-xs text-gray-500">正在加载分组…</p>
      <div v-else class="management-groups grid max-h-40 gap-2 overflow-y-auto">
        <label v-for="group in groups" :key="group.id" class="flex items-start gap-2 text-xs">
          <input v-model="selectedGroupIDs" type="checkbox" :data-group-id="group.id" :value="group.id" :disabled="savingGroups" />
          <span>{{ group.name }} (#{{ group.id }})<span v-if="group.status === 'inactive'" class="ml-1 text-gray-400">已停用</span></span>
        </label>
        <p v-if="!groups.length" class="text-xs text-gray-500">暂无 OpenAI 分组</p>
      </div>
      <p v-if="groupError" role="alert" class="mt-2 text-xs text-red-600">{{ groupError }}</p>
      <div class="mt-3 flex gap-2">
        <button type="button" class="btn btn-primary btn-sm" :disabled="savingGroups || groupsLoading" @click="saveGroups"><Icon v-if="savingGroups" name="refresh" size="xs" class="animate-spin" />保存</button>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="savingGroups" @click="groupEditorOpen = false">取消</button>
      </div>
    </div>

    <div v-if="scheduleOpen" class="space-y-3 rounded border border-gray-200 bg-gray-50 p-3 dark:border-dark-500 dark:bg-dark-700/40">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div><p class="text-xs font-medium">定时检查</p><p class="mt-0.5 text-xs text-gray-500">为当前账号设置模型、思考程度和执行时间</p></div>
        <div class="flex gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="scheduleLoading" title="刷新" aria-label="刷新定时检查" @click="loadPlans"><Icon name="refresh" size="xs" :class="{ 'animate-spin': scheduleLoading }" /></button><button v-if="!newPlanOpen && !editingPlanID" type="button" class="btn btn-secondary btn-sm" :disabled="scheduleLoading || savingPlan" @click="beginCreate"><Icon name="plus" size="xs" />添加定时检查</button></div>
      </div>
      <p v-if="scheduleLoading" class="text-xs text-gray-500">正在加载计划和最新结果…</p>
      <p v-if="scheduleError" role="alert" class="text-xs text-red-600">{{ scheduleError }}</p>
      <p v-if="legacyError" role="alert" class="text-xs text-amber-700">{{ legacyError }}</p>
      <div v-for="plan in legacyPlans" :key="`legacy-${plan.id}`" class="rounded border border-amber-200 p-2 text-xs text-amber-800 dark:border-amber-800 dark:text-amber-300"><p>旧版定时检查 #{{ plan.id }} · {{ plan.enabled ? '仍在运行' : '已暂停' }}</p><p class="mt-1 break-all">{{ plan.model_id }} · {{ plan.cron_expression }}</p><p class="mt-1">旧计划仍由 Core 执行。新建计划前可先暂停，避免重复检查。</p><button v-if="plan.enabled" type="button" class="btn btn-secondary btn-sm mt-2" :disabled="savingPlan" @click="pauseLegacy(plan)">暂停旧计划</button></div>

      <form v-if="newPlanOpen || editingPlanID" class="management-plan-form grid gap-3 rounded border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800" @submit.prevent="savePlan">
        <fieldset class="contents" :disabled="savingPlan">
          <legend class="sr-only">当前账号的定时检查设置</legend>
          <p class="plan-form-wide text-xs font-medium">{{ account.name }} · #{{ account.id }}</p>
          <label class="text-xs text-gray-500">1. 选择模型<select v-model="form.model_id" aria-label="定时检查模型" class="input mt-1 w-full" required><option v-for="model in modelOptions" :key="model.id" :value="model.id">{{ model.name }}</option></select></label>
          <label class="text-xs text-gray-500">2. 思考程度<select v-model="form.reasoning_effort" aria-label="定时检查思考程度" class="input mt-1 w-full"><option v-for="effort in effortOptions" :key="effort.value" :value="effort.value">{{ effort.label }}</option></select><span class="mt-1 block">需所选模型支持；不确定时选「模型默认」。</span></label>
          <label class="plan-form-wide text-xs text-gray-500">检查题目<textarea v-model="form.prompt" aria-label="定时检查题目" class="input mt-1 w-full" rows="3" maxlength="16000" required /><span class="mt-1 block">初始沿用当前账号的题目，开始后按这里保存的内容执行。</span></label>
          <label class="plan-form-wide text-xs text-gray-500">3. 执行频率<select v-model="schedule.frequency" aria-label="执行频率" class="input mt-1 w-full"><option value="minutes">每隔几分钟</option><option value="hourly">每小时</option><option value="daily">每天</option><option value="weekly">每周</option><option value="custom">高级：自定义 Cron</option></select></label>
          <label v-if="schedule.frequency === 'minutes'" class="text-xs text-gray-500">间隔时间<select v-model="schedule.interval" aria-label="间隔时间" class="input mt-1 w-full"><option v-for="minutes in [5, 10, 15, 20, 30]" :key="minutes" :value="minutes">{{ minutes }} 分钟</option></select></label>
          <label v-if="schedule.frequency === 'hourly'" class="text-xs text-gray-500">每小时第几分钟<input v-model.number="schedule.minute" aria-label="每小时第几分钟" class="input mt-1 w-full" type="number" min="0" max="59" step="1" required /></label>
          <label v-if="schedule.frequency === 'weekly'" class="text-xs text-gray-500">星期<select v-model="schedule.weekday" aria-label="星期" class="input mt-1 w-full"><option v-for="(day, index) in weekdays" :key="index" :value="index">{{ day }}</option></select></label>
          <label v-if="schedule.frequency === 'daily' || schedule.frequency === 'weekly'" class="text-xs text-gray-500">执行时间（北京时间）<input v-model="schedule.time" aria-label="执行时间" class="input mt-1 w-full" type="time" required /></label>
          <label v-if="schedule.frequency === 'custom'" class="plan-form-wide text-xs text-gray-500">Cron 表达式<input v-model="schedule.custom" aria-label="Cron 表达式" class="input mt-1 w-full" required placeholder="CRON_TZ=Asia/Shanghai 0 9 * * *" /><span class="mt-1 block">分 时 日 月 周；未指定 CRON_TZ 时使用服务器时区。</span></label>
          <div class="plan-form-wide rounded bg-gray-50 p-2 text-xs leading-relaxed dark:bg-dark-700" role="status"><p>{{ scheduleDescription(schedule) }}{{ schedule.frequency === 'custom' ? '' : '（北京时间）' }}</p><p class="mt-1 text-gray-500">{{ form.enabled ? '开始后按上述时间自动执行，首次执行等待下一个时间点。' : '保存后保持暂停，可点击「继续」恢复执行。' }}关闭页面后仍会继续，最近保留 3 条结果。暂停只停止后续执行，进行中的检查会继续完成。</p></div>
          <div class="plan-form-wide flex flex-wrap items-end gap-2"><button type="submit" class="btn btn-primary btn-sm" :disabled="savingPlan || !form.model_id.trim() || !form.prompt.trim() || !cronExpression"><Icon :name="savingPlan ? 'refresh' : 'play'" size="xs" :class="{ 'animate-spin': savingPlan }" />{{ savingPlan ? '正在保存…' : editingPlanID ? '保存设置' : '开始定时检查' }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" @click="cancelPlanForm">取消</button></div>
        </fieldset>
      </form>

      <p v-if="!scheduleLoading && !plans.length && !newPlanOpen" class="py-2 text-center text-xs text-gray-500">暂未设置定时检查</p>
      <article v-for="plan in plans" :key="plan.id" :data-schedule-plan="plan.id" class="rounded border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0"><div class="flex flex-wrap items-center gap-2 text-xs"><strong>定时检查 #{{ plan.id }}</strong><span :class="plan.enabled ? 'text-green-700 dark:text-green-400' : 'text-gray-400'">{{ plan.enabled ? '定时运行中' : '已暂停' }}</span></div><p class="mt-1 break-all text-xs text-gray-500">{{ plan.model_id || '默认模型' }} · 思考：{{ effortLabel(plan.reasoning_effort) }}</p><p class="mt-1 break-all text-xs text-gray-500">{{ scheduleDescription(parseCheckSchedule(plan.cron_expression)) }}</p><p v-if="plan.enabled" class="mt-1 text-xs text-gray-500">下次执行（北京时间）：{{ formatTime(plan.next_run_at) }}</p></div>
          <div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" @click="toggleEnabled(plan)">{{ plan.enabled ? '暂停' : '继续' }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" title="编辑" :aria-label="`编辑计划 ${plan.id}`" @click="beginEdit(plan)"><Icon name="edit" size="xs" /></button><button type="button" class="btn btn-secondary btn-sm text-red-600" :disabled="deletingPlanID === plan.id" title="删除" :aria-label="`删除计划 ${plan.id}`" @click="removePlan(plan)"><Icon name="trash" size="xs" /></button></div>
        </div>
        <div class="mt-3 rounded bg-gray-50 p-2 text-xs dark:bg-dark-700">
          <template v-if="latestResults[plan.id]"><div class="flex flex-wrap justify-between gap-2"><span :class="statusClass(latestResults[plan.id]!.status)">{{ statusText(latestResults[plan.id]!.status) }}<span v-if="latestResults[plan.id]!.latency_ms"> · {{ latestResults[plan.id]!.latency_ms }}ms</span></span><time>{{ formatTime(latestResults[plan.id]!.finished_at || latestResults[plan.id]!.started_at) }}</time></div><p class="mt-1 line-clamp-4 whitespace-pre-wrap break-words text-gray-700 dark:text-gray-300">{{ latestResults[plan.id]!.error_message || latestResults[plan.id]!.response_text || '无返回内容' }}</p></template>
          <p v-else class="text-gray-500">暂无检查结果 · {{ formatTime(plan.last_run_at) }}</p>
        </div>
        <AccountCheckResult v-if="latestResults[plan.id]?.html && displayResults[plan.id]" embedded :account="account" :prompt="plan.prompt || ''" :model="plan.model_id" :state="latestResults[plan.id]!.status === 'success' ? 'succeeded' : 'failed'" :result="displayResults[plan.id]" :error="latestResults[plan.id]!.error_message" :elapsed="Math.round(latestResults[plan.id]!.latency_ms / 1000)" />
        <button type="button" :data-plan-history="plan.id" class="mt-2 inline-flex items-center gap-1 text-xs text-primary-600 hover:underline dark:text-primary-400" @click="toggleHistory(plan)"><Icon name="chevronDown" size="xs" :class="{ 'rotate-180': historyOpen[plan.id] }" />{{ historyOpen[plan.id] ? '收起记录' : '最近 3 条' }}</button>
        <div v-if="historyOpen[plan.id]" class="mt-2 space-y-2">
          <p v-if="historyLoading[plan.id]" class="text-xs text-gray-500">正在加载记录…</p>
          <div v-for="result in histories[plan.id]" :key="result.id" class="border-l-2 border-gray-200 pl-3 text-xs dark:border-dark-500"><div class="flex flex-wrap justify-between gap-2"><span :class="statusClass(result.status)">{{ statusText(result.status) }}</span><time class="text-gray-500">{{ formatTime(result.finished_at || result.started_at) }}</time></div><p class="mt-1 max-h-28 overflow-y-auto whitespace-pre-wrap break-words text-gray-700 dark:text-gray-300">{{ result.error_message || result.response_text || '无返回内容' }}</p></div>
          <p v-if="!historyLoading[plan.id] && !histories[plan.id]?.length" class="text-xs text-gray-500">暂无记录</p>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.check-management { margin-top: 10px; padding-top: 9px; border-top: 1px solid var(--line, #e2e9e8); }
.check-management .btn { font-size: 11px; padding: 4px 8px; border-radius: 6px; min-height: 27px; }
.management-actions { gap: 6px; }
.management-actions .btn { font-size: 10px; padding: 4px 5px; gap: 4px; }
.check-management .input { min-width: 0; width: 100%; font-size: 12px; }
.management-groups, .management-plan-form { grid-template-columns: minmax(0, 1fr); }
.plan-form-wide { grid-column: 1 / -1; min-width: 0; }
.management-groups label { overflow-wrap: anywhere; }
.management-groups input { flex-shrink: 0; margin-top: 2px; }
@container (min-width: 480px) {
  .management-groups { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .management-plan-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
