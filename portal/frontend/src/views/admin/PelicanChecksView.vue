<template>
  <AppLayout>
    <div class="pelican-page">
      <header class="page-heading">
        <div>
          <div class="title-row">
            <h1>鹈鹕绘图检查</h1>
            <span class="monitor-state" :data-enabled="data?.config.enabled ? 'true' : 'false'">
              {{ data?.config.enabled ? '检测已开启' : '检测已关闭' }}
            </span>
          </div>
          <p>自定义绘图检查 · 作品画廊</p>
        </div>
        <div class="heading-actions">
          <label class="auto-refresh">
            <Toggle v-model="autoRefresh" />
            <span>自动刷新</span>
          </label>
          <button class="btn btn-secondary btn-sm" type="button" :disabled="loading" @click="refreshStatus">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            刷新
          </button>
        </div>
      </header>

      <section v-if="canConfigure" class="config-section">
        <div class="section-heading">
          <div>
            <h2>检查配置</h2>
            <p>配置检查题目、模型及调用分组，仅管理员可编辑。</p>
          </div>
          <span v-if="data?.config.updated_at" class="updated-at">更新于 {{ formatDateTime(data.config.updated_at) }}</span>
        </div>
        <div class="config-form">
          <label class="form-field">
            <span>Core 分组</span>
            <select v-model="selectedGroupId" class="input" :disabled="groupsLoading || saving" @change="handleGroupChange">
              <option :value="null">请选择分组</option>
              <option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option>
            </select>
          </label>
          <label class="form-field">
            <span>API Key</span>
            <select v-model="selectedKeyId" class="input" :disabled="!selectedGroupId || keysLoading || saving" @change="loadModels">
              <option :value="null">{{ keysLoading ? '正在读取...' : '请选择 API Key' }}</option>
              <option v-for="key in keys" :key="key.id" :value="key.id">
                {{ key.name || `Key #${key.id}` }} · {{ key.preview }}
              </option>
            </select>
          </label>
          <label class="enable-field">
            <span>开启检测</span>
            <Toggle v-model="enabled" :disabled="saving" />
          </label>
          <div class="config-actions">
            <button class="btn btn-primary btn-sm" type="button" :disabled="saving || triggering || !validPrompt || !selectedGroupId || !selectedKeyId" @click="saveConfig">
              <Icon name="check" size="sm" />
              {{ saving ? '保存中...' : '保存配置' }}
            </button>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="running || saving || !validPrompt || !enabled || !selectedGroupId || !selectedKeyId" @click="runNow">
              <Icon name="play" size="sm" />
              {{ running ? '检测中...' : '立即检查' }}
            </button>
          </div>
        </div>
        <p v-if="saveState" class="save-state" role="status">{{ saveState }}</p>
        <div class="prompt-config">
          <div class="model-settings">
          <label class="form-field">
            <span>检查模型</span>
            <select v-model="model" class="input" :disabled="saving || triggering || modelsLoading">
              <option v-for="item in modelOptions" :key="item" :value="item">{{ item }}</option>
            </select>
            <small>{{ modelsLoading ? '正在加载模型...' : '模型列表来自所选 API Key；保留当前配置的模型。' }}</small>
          </label>
          <label class="form-field">
            <span>思考深度</span>
            <select v-model="reasoningEffort" class="input" :disabled="saving || triggering">
              <option value="none">不思考（none）</option>
              <option value="minimal">最少（minimal）</option>
              <option value="low">低（low）</option>
              <option value="medium">中（medium）</option>
              <option value="high">高（high）</option>
              <option value="xhigh">极高（xhigh）</option>
            </select>
            <small>较高深度通常耗时更长；支持的深度取决于模型及上游。</small>
          </label>
          </div>
          <label class="form-field">
            <span>检查题目</span>
            <textarea v-model="prompt" class="input" rows="4" maxlength="10000" placeholder="描述需要生成的绘图内容" :disabled="saving || triggering"></textarea>
            <small>题目、模型和思考深度修改后自动保存，用于后续检查；当前运行的检查不受影响。</small>
          </label>
        </div>
      </section>

      <section class="latest-section">
        <div class="section-heading">
          <div><h2>最近作品</h2><p>每 30 分钟自动生成，点击作品可放大查看。</p></div>
          <span v-if="canConfigure" class="updated-at">{{ running ? '正在生成作品' : `下次生成：${nextCheck}` }}</span>
        </div>
        <div v-if="artworkItems.length" class="artwork-grid">
          <article v-for="artwork in artworkItems" :key="artwork.id" class="artwork-card">
            <button
              v-if="canConfigure"
              class="artwork-delete"
              type="button"
              :title="`删除作品 #${artwork.id}`"
              :aria-label="`删除作品 #${artwork.id}`"
              @click="requestDeleteArtwork(artwork)"
            >
              <Icon name="trash" size="sm" />
            </button>
            <button class="artwork-preview" type="button" :title="`查看 ${formatDateTime(artwork.finished_at)} 的作品`" @click="openArtwork(artwork)">
              <iframe tabindex="-1" aria-hidden="true" sandbox="allow-scripts" referrerpolicy="no-referrer" :srcdoc="artwork.html"></iframe>
              <span class="preview-action"><Icon name="eye" size="sm" /></span>
            </button>
            <div class="artwork-card-meta">
              <div>
                <strong>{{ artwork.group_name }}</strong>
                <span>{{ artwork.api_key_name || `Key #${artwork.api_key_id}` }}</span>
                <span v-if="canConfigure" class="artwork-account">
                  生成账号：{{ artwork.account_name || (artwork.account_id ? `账号 #${artwork.account_id}` : '历史记录暂未识别') }}<template v-if="artwork.account_name && artwork.account_id"> · #{{ artwork.account_id }}</template>
                </span>
              </div>
              <div>
                <time>{{ formatDateTime(artwork.finished_at) }}</time>
                <span>{{ formatDuration(artwork.latency_ms) }}</span>
              </div>
            </div>
          </article>
        </div>
        <div v-else class="empty-state">暂无成功作品</div>
      </section>
    </div>

    <BaseDialog :show="previewOpen" title="鹈鹕绘图作品" width="full" @close="previewOpen = false">
      <iframe v-if="selectedArtwork" class="large-preview" title="鹈鹕绘图作品大图" sandbox="allow-scripts" referrerpolicy="no-referrer" :srcdoc="selectedArtwork.html"></iframe>
    </BaseDialog>
    <ConfirmDialog
      :show="Boolean(deletingArtwork)"
      title="删除作品"
      :message="`确认删除作品 #${deletingArtwork?.id ?? ''}？删除后无法恢复。`"
      confirm-text="删除"
      :danger="true"
      :confirm-loading="deleting"
      @confirm="confirmDeleteArtwork"
      @cancel="deletingArtwork = null"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { onBeforeRouteLeave } from 'vue-router'
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  deletePelicanArtwork,
  getPelicanCheckOptions,
  getPelicanChecks,
  runPelicanCheckNow,
  updatePelicanCheckConfig,
  type PelicanCheckResponse,
  type PelicanArtwork,
  type PelicanGroupOption,
  type PelicanKeyOption
} from '@/api/admin/pelicanChecks'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const authStore = useAuthStore()
const data = ref<PelicanCheckResponse | null>(null)
const groups = ref<PelicanGroupOption[]>([])
const keys = ref<PelicanKeyOption[]>([])
const selectedGroupId = ref<number | null>(null)
const selectedKeyId = ref<number | null>(null)
const enabled = ref(false)
const model = ref('gpt-6-astra')
const reasoningEffort = ref('high')
const models = ref<string[]>([])
const modelsLoading = ref(false)
const modelOptions = computed(() => [...new Set([model.value, ...models.value].filter(Boolean))])
let modelRequest = 0
const prompt = ref('生成 html，内容是 svg 绘制鹈鹕骑自行车 2D 动画，不用进行测试')
const configReady = ref(false)
const saveState = ref('')
const draftKey = `pelican-check-draft:${authStore.user?.id ?? 'admin'}`
let autoSaveTimer: number | undefined
let pendingSave: Promise<boolean> | undefined
const draftValues = () => ({ model: model.value, prompt: prompt.value, reasoning_effort: reasoningEffort.value })
function rememberDraft() {
  try { sessionStorage.setItem(draftKey, JSON.stringify(draftValues())) } catch { /* Storage can be disabled. */ }
}
function restoreDraft() {
  try {
    const draft = JSON.parse(sessionStorage.getItem(draftKey) || 'null')
    if (draft && typeof draft.model === 'string' && typeof draft.prompt === 'string' && typeof draft.reasoning_effort === 'string') {
      model.value = draft.model
      prompt.value = draft.prompt
      reasoningEffort.value = draft.reasoning_effort
      saveState.value = '已恢复未保存的修改'
    }
  } catch { /* Ignore invalid browser drafts. */ }
}
const validPrompt = computed(() => Boolean(model.value.trim() && prompt.value.trim()))
const loading = ref(false)
const groupsLoading = ref(false)
const keysLoading = ref(false)
const saving = ref(false)
const triggering = ref(false)
const autoRefresh = ref(localStorage.getItem('pelican-check-auto-refresh') !== 'false')
const previewOpen = ref(false)
const selectedArtwork = ref<PelicanArtwork | null>(null)
const deletingArtwork = ref<PelicanArtwork | null>(null)
const deleting = ref(false)
const now = ref(Date.now())
const serverClockOffset = ref(0)
const statusError = ref(false)
let refreshTimer: number | undefined
let clockTimer: number | undefined
let runPollTimer: number | undefined

const canConfigure = computed(() => authStore.isFullAdmin)
const running = computed(() => triggering.value || Boolean(data.value?.summary.running))
const artworkItems = computed(() => data.value?.artworks ?? (data.value?.latest_artwork ? [data.value.latest_artwork] : []))
const nextCheck = computed(() => {
  if (!data.value?.config.enabled) return '已关闭'
  if (statusError.value) return '状态更新失败'
  if (data.value.summary.running) return '检测中'
  if (!data.value.scheduler_healthy) return '调度服务未响应'
  if (!data.value.next_run_at) return '等待调度'
  const seconds = Math.ceil((Date.parse(data.value.next_run_at) - now.value - serverClockOffset.value) / 1000)
  if (seconds <= -30) return '调度延迟'
  if (seconds <= 0) return '等待执行'
  return `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`
})

async function loadStatus(syncConfig = false) {
  if (loading.value) return
  loading.value = true
  try {
    const response = await getPelicanChecks()
    data.value = response
    statusError.value = false
    serverClockOffset.value = Date.parse(response.generated_at) - Date.now()
    if (syncConfig && canConfigure.value) {
      selectedGroupId.value = response.config.group_id
      selectedKeyId.value = response.config.api_key_id
      enabled.value = response.config.enabled
      model.value = response.config.model ?? model.value
      reasoningEffort.value = response.config.reasoning_effort ?? 'high'
      prompt.value = response.config.prompt ?? prompt.value
    }
  } catch (error) {
    statusError.value = true
    appStore.showError(extractApiErrorMessage(error, '鹈鹕绘图检查加载失败'))
  } finally {
    loading.value = false
  }
}

function refreshStatus() {
  void loadStatus()
}

function openArtwork(artwork: PelicanArtwork) {
  selectedArtwork.value = artwork
  previewOpen.value = true
}

function requestDeleteArtwork(artwork: PelicanArtwork) {
  if (!canConfigure.value) return
  deletingArtwork.value = artwork
}

async function confirmDeleteArtwork() {
  const artwork = deletingArtwork.value
  if (!artwork || deleting.value || !canConfigure.value) return
  deleting.value = true
  try {
    await deletePelicanArtwork(artwork.id)
    if (selectedArtwork.value?.id === artwork.id) {
      selectedArtwork.value = null
      previewOpen.value = false
    }
    deletingArtwork.value = null
    appStore.showSuccess('作品已删除')
    await loadStatus()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '作品删除失败'))
  } finally {
    deleting.value = false
  }
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    groups.value = (await getPelicanCheckOptions()).groups
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, 'Core 分组加载失败'))
  } finally {
    groupsLoading.value = false
  }
}

async function loadKeys(groupId: number | null) {
  keys.value = []
  if (!groupId) return
  keysLoading.value = true
  try {
    keys.value = (await getPelicanCheckOptions(groupId)).keys
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, 'API Key 加载失败'))
  } finally {
    keysLoading.value = false
  }
}

async function loadModels() {
  const request = ++modelRequest
  models.value = []
  modelsLoading.value = false
  if (!selectedGroupId.value || !selectedKeyId.value) return
  modelsLoading.value = true
  try {
    const options = await getPelicanCheckOptions(selectedGroupId.value, selectedKeyId.value)
    if (request === modelRequest) models.value = options.models ?? []
  } catch (error) {
    if (request === modelRequest) appStore.showError(extractApiErrorMessage(error, '模型列表加载失败，可继续使用当前模型'))
  } finally {
    if (request === modelRequest) modelsLoading.value = false
  }
}

async function handleGroupChange() {
  selectedKeyId.value = null
  void loadModels()
  await loadKeys(selectedGroupId.value)
}

function saveConfig(): Promise<boolean> {
  if (pendingSave) return pendingSave
  if (!selectedGroupId.value || !selectedKeyId.value || !validPrompt.value) {
    saveState.value = '尚未保存：请填写题目并选择分组、Key 和模型'
    return Promise.resolve(false)
  }
  const input = { enabled: enabled.value, group_id: selectedGroupId.value, api_key_id: selectedKeyId.value, ...draftValues(), model: model.value.trim(), prompt: prompt.value.trim() }
  const snapshot = JSON.stringify(draftValues())
  saving.value = true
  saveState.value = '保存中…'
  pendingSave = (async () => {
    try {
      const saved = await updatePelicanCheckConfig(input)
      if (data.value) data.value.config = { ...data.value.config, ...input, ...saved }
      if (JSON.stringify(draftValues()) === snapshot) {
        saveState.value = '已自动保存'
        try { sessionStorage.removeItem(draftKey) } catch { /* Storage can be disabled. */ }
      }
      return true
    } catch (error) {
      saveState.value = '保存失败，修改已保留，请点击保存配置重试'
      rememberDraft()
      appStore.showError(extractApiErrorMessage(error, '检查配置保存失败'))
      return false
    } finally {
      saving.value = false
      pendingSave = undefined
    }
  })()
  return pendingSave
}

watch([model, prompt, reasoningEffort], () => {
  if (!configReady.value || !canConfigure.value) return
  rememberDraft()
  saveState.value = '有修改，等待保存…'
  if (autoSaveTimer) window.clearTimeout(autoSaveTimer)
  autoSaveTimer = window.setTimeout(() => { void saveConfig() }, 800)
}, { flush: 'sync' })

onBeforeRouteLeave(async () => {
  if (!canConfigure.value || !configReady.value || !saveState.value || saveState.value === '已自动保存') return
  if (autoSaveTimer) window.clearTimeout(autoSaveTimer)
  return await saveConfig()
})

async function runNow() {
  if (running.value || saving.value || !validPrompt.value || !enabled.value || !selectedGroupId.value || !selectedKeyId.value) return
  triggering.value = true
  try {
    const config = data.value?.config
    if (!config || config.enabled !== enabled.value || config.group_id !== selectedGroupId.value || config.api_key_id !== selectedKeyId.value || config.reasoning_effort !== reasoningEffort.value || config.model !== model.value.trim() || config.prompt !== prompt.value.trim()) {
      await updatePelicanCheckConfig({ enabled: enabled.value, group_id: selectedGroupId.value, api_key_id: selectedKeyId.value, reasoning_effort: reasoningEffort.value, model: model.value.trim(), prompt: prompt.value.trim() })
    }
    const result = await runPelicanCheckNow()
    appStore.showSuccess('已开始鹈鹕绘图检查')
    startRunPolling(result.run_id)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '启动检查失败'))
    triggering.value = false
    await loadStatus()
  }
}

function startRunPolling(runId: number) {
  if (runPollTimer) window.clearTimeout(runPollTimer)
  triggering.value = true
  const deadline = Date.now() + 20 * 60 * 1000
  const poll = async () => {
    await loadStatus()
    const finished = data.value && data.value.latest_run_id >= runId && data.value.active_run_id !== runId
    if (finished || Date.now() >= deadline) {
      triggering.value = false
      runPollTimer = undefined
      return
    }
    runPollTimer = window.setTimeout(() => void poll(), 2000)
  }
  runPollTimer = window.setTimeout(() => void poll(), 500)
}

function restartRefresh() {
  if (refreshTimer) window.clearInterval(refreshTimer)
  refreshTimer = undefined
  refreshTimer = window.setInterval(() => {
    if (autoRefresh.value || data.value?.summary.running) void loadStatus()
  }, 5000)
}

watch(autoRefresh, (value) => {
  localStorage.setItem('pelican-check-auto-refresh', String(value))
  restartRefresh()
})

onMounted(async () => {
  if (canConfigure.value) {
    await Promise.all([loadStatus(true), loadGroups()])
    await loadKeys(selectedGroupId.value)
    await loadModels()
    restoreDraft()
    configReady.value = !statusError.value
    if (saveState.value) void saveConfig()
  } else {
    await loadStatus()
  }
  restartRefresh()
  clockTimer = window.setInterval(() => { now.value = Date.now() }, 1000)
})

onBeforeUnmount(() => {
  if (autoSaveTimer) window.clearTimeout(autoSaveTimer)
  if (refreshTimer) window.clearInterval(refreshTimer)
  if (clockTimer) window.clearInterval(clockTimer)
  if (runPollTimer) window.clearTimeout(runPollTimer)
})

function formatDuration(milliseconds: number) {
  if (!milliseconds) return '--'
  return milliseconds < 1000 ? `${milliseconds} ms` : `${(milliseconds / 1000).toFixed(1)} 秒`
}

function formatDateTime(value?: string | null) {
  if (!value) return '尚未检测'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}

</script>

<style scoped>
.save-state{margin-top:12px;font-size:12px;color:#64748b}.model-settings{display:grid;gap:12px}.prompt-config{display:grid;grid-template-columns:minmax(180px,1fr) minmax(300px,3fr);gap:16px;margin-top:16px;padding-top:16px;border-top:1px solid #e2e8f0}.prompt-config small{color:#64748b;font-size:11px}.prompt-config textarea{resize:vertical;min-height:100px}.dark .prompt-config{border-color:#334155}@media(max-width:640px){.prompt-config{grid-template-columns:1fr}}
.pelican-page{display:grid;gap:20px;color:#0f172a}.page-heading,.section-heading,.heading-actions,.auto-refresh,.title-row{display:flex;align-items:center}.page-heading,.section-heading{justify-content:space-between}.page-heading{min-height:44px;border-bottom:1px solid #e2e8f0;padding-bottom:14px}.title-row{gap:10px}.page-heading h1{font-size:20px;font-weight:750}.page-heading p,.section-heading p{margin-top:3px;color:#64748b;font-size:12px}.monitor-state{border-radius:4px;background:#f1f5f9;padding:3px 7px;color:#64748b;font-size:10px;font-weight:700}.monitor-state[data-enabled=true]{background:#ecfdf5;color:#047857}.heading-actions{gap:14px}.auto-refresh{gap:8px;font-size:12px;color:#475569}.config-section{border:1px solid #dbe5ef;border-radius:8px;background:#fff;padding:16px}.config-section .section-heading{margin-bottom:14px}.updated-at{color:#94a3b8;font-size:10px}.config-form{display:grid;grid-template-columns:minmax(180px,1fr) minmax(220px,1.4fr) auto auto;align-items:end;gap:12px}.form-field{display:grid;gap:6px;min-width:0}.form-field>span,.enable-field>span{color:#475569;font-size:11px;font-weight:650}.form-field .input{width:100%}.enable-field{display:grid;justify-items:center;gap:8px;min-width:72px;padding-bottom:7px}.config-actions{display:flex;gap:8px;padding-bottom:1px}.latest-section{border-top:1px solid #e2e8f0;padding-top:16px}.section-heading{gap:16px;margin-bottom:12px}.section-heading h2{font-size:14px;font-weight:750}.artwork-grid{display:flex;flex-wrap:wrap;gap:14px}.artwork-card{position:relative;width:min(100%,380px);overflow:hidden;border:1px solid #dbe5ef;border-radius:8px;background:#fff}.artwork-preview{position:relative;display:block;width:100%;aspect-ratio:16/9;overflow:hidden;border:0;border-bottom:1px solid #e2e8f0;background:#f8fafc;padding:0;cursor:pointer}.artwork-preview iframe{position:absolute;inset:0;width:400%;height:400%;border:0;background:#fff;pointer-events:none;transform:scale(.25);transform-origin:0 0}.preview-action,.artwork-delete{position:absolute;top:8px;display:grid;width:28px;height:28px;place-items:center;border:1px solid #dbe5ef;border-radius:6px;background:rgba(255,255,255,.92);transition:opacity .15s ease}.preview-action{right:8px;color:#475569;opacity:0}.artwork-delete{left:8px;z-index:2;color:#dc2626;cursor:pointer}.artwork-delete:hover{border-color:#fecaca;background:#fff1f2}.artwork-preview:hover .preview-action,.artwork-preview:focus-visible .preview-action{opacity:1}.artwork-card-meta{display:flex;justify-content:space-between;gap:12px;padding:10px 12px}.artwork-card-meta>div{display:grid;min-width:0;gap:3px}.artwork-card-meta>div:first-child{flex:1}.artwork-card-meta>div:last-child{text-align:right}.artwork-card-meta strong,.artwork-card-meta time{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:11px}.artwork-card-meta span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:#64748b;font-size:10px}.artwork-card-meta .artwork-account{margin-top:2px;color:#0369a1;font-weight:650}.large-preview{display:block;width:100%;height:72dvh;border:1px solid #e2e8f0;border-radius:6px;background:#fff}.empty-state{display:grid;min-height:160px;place-items:center;border:1px dashed #cbd5e1;border-radius:8px;color:#64748b;font-size:13px}.dark .pelican-page{color:#f8fafc}.dark .page-heading,.dark .latest-section{border-color:#334155}.dark .config-section,.dark .artwork-card{border-color:#334155;background:#172033}.dark .monitor-state{background:#273449}.dark .artwork-preview{border-color:#334155;background:#0f172a}.dark .preview-action,.dark .artwork-delete{border-color:#475569;background:rgba(23,32,51,.92)}.dark .preview-action{color:#cbd5e1}.dark .artwork-delete{color:#fda4af}.dark .artwork-card-meta .artwork-account{color:#7dd3fc}.dark .page-heading p,.dark .section-heading p,.dark .auto-refresh,.dark .form-field>span,.dark .enable-field>span{color:#94a3b8}
@media(max-width:1100px){.config-form{grid-template-columns:1fr 1fr}}
@media(max-width:640px){.pelican-page{gap:16px}.page-heading,.section-heading{align-items:flex-start;flex-direction:column}.heading-actions{width:100%;justify-content:space-between}.config-form{grid-template-columns:1fr}.enable-field{justify-items:start}.config-actions{flex-wrap:wrap}.artwork-card{width:100%}}
</style>
