<script setup lang="ts">
import { reactive, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import {
  createScheduledCheck,
  deleteScheduledCheck,
  listScheduledCheckResults,
  listScheduledChecks,
  updateCheckAccountGroups,
  updateScheduledCheck,
  type CheckAccount,
  type CheckGroupOption,
  type ScheduledCheckPlan,
  type ScheduledCheckResult
} from '@/api/admin/accountChecks'

const props = defineProps<{
  account: CheckAccount
  groups: CheckGroupOption[]
  groupsLoading: boolean
  models: Array<{ id: string; display_name: string }>
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
const latestResults = reactive<Record<number, ScheduledCheckResult | null>>({})
const histories = reactive<Record<number, ScheduledCheckResult[]>>({})
const historyOpen = reactive<Record<number, boolean>>({})
const historyLoading = reactive<Record<number, boolean>>({})
const editingPlanID = ref<number | null>(null)
const savingPlan = ref(false)
const deletingPlanID = ref<number | null>(null)
const newPlanOpen = ref(false)
const form = reactive({ model_id: 'gpt-6-astra', cron_expression: '0 * * * *', enabled: true })

const errorMessage = (error: unknown) => (error as { message?: string })?.message || '操作失败，请重试'
const formatTime = (value: string | null | undefined) => {
  if (!value) return '尚未执行'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : new Intl.DateTimeFormat('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date)
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
    latestResults[plan.id] = results[0] || null
  } catch (error) {
    scheduleError.value = errorMessage(error)
    latestResults[plan.id] = null
  }
}

async function loadPlans() {
  scheduleLoading.value = true
  scheduleError.value = ''
  try {
    plans.value = await listScheduledChecks(props.account.id)
    await Promise.all(plans.value.map(loadLatest))
  } catch (error) {
    scheduleError.value = errorMessage(error)
  } finally {
    scheduleLoading.value = false
  }
}

async function toggleSchedule() {
  scheduleOpen.value = !scheduleOpen.value
  if (scheduleOpen.value && !plans.value.length) await loadPlans()
}

function beginCreate() {
  editingPlanID.value = null
  form.model_id = 'gpt-6-astra'
  form.cron_expression = '0 * * * *'
  form.enabled = true
  newPlanOpen.value = true
}

function beginEdit(plan: ScheduledCheckPlan) {
  editingPlanID.value = plan.id
  form.model_id = plan.model_id || 'gpt-6-astra'
  form.cron_expression = plan.cron_expression
  form.enabled = plan.enabled
  newPlanOpen.value = false
}

function cancelPlanForm() {
  editingPlanID.value = null
  newPlanOpen.value = false
}

async function savePlan() {
  if (savingPlan.value || !form.model_id.trim() || !form.cron_expression.trim()) return
  savingPlan.value = true
  scheduleError.value = ''
  try {
    if (editingPlanID.value) {
      const updated = await updateScheduledCheck(props.account.id, editingPlanID.value, { ...form })
      const index = plans.value.findIndex(plan => plan.id === updated.id)
      if (index >= 0) plans.value[index] = updated
    } else {
      const created = await createScheduledCheck(props.account.id, { ...form })
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
        <div><p class="text-xs font-medium">定时检查计划</p><p class="mt-0.5 text-xs text-gray-500">每个计划仅保留最近 3 条结果</p></div>
        <div class="flex gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="scheduleLoading" title="刷新" aria-label="刷新定时检查" @click="loadPlans"><Icon name="refresh" size="xs" :class="{ 'animate-spin': scheduleLoading }" /></button><button type="button" class="btn btn-primary btn-sm" :disabled="scheduleLoading || savingPlan" @click="beginCreate"><Icon name="plus" size="xs" />新增</button></div>
      </div>
      <p v-if="scheduleLoading" class="text-xs text-gray-500">正在加载计划和最新结果…</p>
      <p v-if="scheduleError" role="alert" class="text-xs text-red-600">{{ scheduleError }}</p>

      <div v-if="newPlanOpen || editingPlanID" class="management-plan-form grid gap-3 rounded border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800">
        <label class="text-xs text-gray-500">模型<input v-model="form.model_id" class="input mt-1 w-full" :list="`account-check-schedule-models-${account.id}`" /></label>
        <datalist :id="`account-check-schedule-models-${account.id}`"><option v-for="model in models" :key="model.id" :value="model.id">{{ model.display_name }}</option></datalist>
        <label class="text-xs text-gray-500">Cron 表达式<input v-model="form.cron_expression" class="input mt-1 w-full" required placeholder="0 * * * *" /></label>
        <div class="flex items-end gap-2"><button type="button" class="btn btn-primary btn-sm" :disabled="savingPlan || !form.model_id.trim() || !form.cron_expression.trim()" @click="savePlan"><Icon v-if="savingPlan" name="refresh" size="xs" class="animate-spin" />保存</button><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" @click="cancelPlanForm">取消</button></div>
      </div>

      <p v-if="!scheduleLoading && !plans.length && !newPlanOpen" class="py-2 text-center text-xs text-gray-500">暂未设置定时检查</p>
      <article v-for="plan in plans" :key="plan.id" :data-schedule-plan="plan.id" class="rounded border border-gray-200 bg-white p-3 dark:border-dark-500 dark:bg-dark-800">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0"><div class="flex flex-wrap items-center gap-2 text-xs"><strong>计划 #{{ plan.id }}</strong><span :class="plan.enabled ? 'text-green-700 dark:text-green-400' : 'text-gray-400'">{{ plan.enabled ? '已启用' : '已暂停' }}</span></div><p class="mt-1 break-all text-xs text-gray-500">{{ plan.model_id || '默认模型' }} · {{ plan.cron_expression }} · 下次 {{ formatTime(plan.next_run_at) }}</p></div>
          <div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" @click="toggleEnabled(plan)">{{ plan.enabled ? '暂停' : '启用' }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="savingPlan" title="编辑" :aria-label="`编辑计划 ${plan.id}`" @click="beginEdit(plan)"><Icon name="edit" size="xs" /></button><button type="button" class="btn btn-secondary btn-sm text-red-600" :disabled="deletingPlanID === plan.id" title="删除" :aria-label="`删除计划 ${plan.id}`" @click="removePlan(plan)"><Icon name="trash" size="xs" /></button></div>
        </div>
        <div class="mt-3 rounded bg-gray-50 p-2 text-xs dark:bg-dark-700">
          <template v-if="latestResults[plan.id]"><div class="flex flex-wrap justify-between gap-2"><span :class="statusClass(latestResults[plan.id]!.status)">{{ statusText(latestResults[plan.id]!.status) }}<span v-if="latestResults[plan.id]!.latency_ms"> · {{ latestResults[plan.id]!.latency_ms }}ms</span></span><time>{{ formatTime(latestResults[plan.id]!.finished_at || latestResults[plan.id]!.started_at) }}</time></div><p class="mt-1 line-clamp-4 whitespace-pre-wrap break-words text-gray-700 dark:text-gray-300">{{ latestResults[plan.id]!.error_message || latestResults[plan.id]!.response_text || '无返回内容' }}</p></template>
          <p v-else class="text-gray-500">暂无检查结果 · {{ formatTime(plan.last_run_at) }}</p>
        </div>
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
.management-groups label { overflow-wrap: anywhere; }
.management-groups input { flex-shrink: 0; margin-top: 2px; }
@container (min-width: 480px) {
  .management-groups { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .management-plan-form { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
