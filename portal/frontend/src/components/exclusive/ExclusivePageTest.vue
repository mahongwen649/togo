<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { exclusiveChannels as api, type ChannelAccount, type ChannelPageTest } from '@/api/exclusiveChannels'
import { pagePreviewDocument } from './pagePreview'

const props = defineProps<{ channelId: number; account: ChannelAccount; canTest?: boolean; compact?: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const result = ref<ChannelPageTest | null>(null)
const open = ref(false), sending = ref(false), loading = ref(false), error = ref('')
const preview = ref<HTMLElement | null>(null)
let timer: ReturnType<typeof setTimeout> | undefined
let disposed = false, version = 0
const current = computed(() => result.value || props.account.page_test)
const running = computed(() => current.value?.state === 'running')
const blocked = computed(() => running.value || current.value?.state === 'uncertain')
const source = computed(() => result.value?.html ? pagePreviewDocument(result.value.html) : '')
const inlineSource = computed(() => result.value?.html ? pagePreviewDocument(result.value.html, 'fit') : '')
const stamp = (value?: string | null) => value ? new Date(value).toLocaleString('zh-CN', { hour12: false }) : '尚未生成'
const status = computed(() => ({ none: '尚未测试', running: '生成中', succeeded: '已生成', failed: '生成失败', uncertain: '待核查' })[current.value?.state || 'none'])
const message = (e: unknown) => {
  const value = e as { response?: { data?: { message?: string } }; message?: string }
  return value.response?.data?.message || value.message || '加载失败，请重试'
}
async function refresh() {
  clearTimeout(timer)
  if (loading.value) return
  const ticket = ++version
  loading.value = true
  try {
    const value = await api.downgradeTest(props.channelId, props.account.id)
    if (disposed || ticket !== version) return
    const previous = current.value?.state
    result.value = value; error.value = ''
    if (previous === 'running' && value.state !== 'running') emit('changed')
  } catch (e) { if (!disposed && ticket === version) error.value = message(e) }
  finally {
    if (!disposed && ticket === version) {
      loading.value = false
      if (running.value) timer = setTimeout(refresh, 2500)
    }
  }
}
watch(
  [
    () => props.channelId,
    () => props.account.id,
    () => props.account.page_test?.has_page,
    () => props.account.page_test?.state,
    () => props.account.page_test?.generated_at,
    () => props.account.page_test?.error
  ],
  ([channelId, accountId], previous) => {
    const metadata = props.account.page_test
    const accountChanged = !previous || channelId !== previous[0] || accountId !== previous[1]
    if (accountChanged) {
      clearTimeout(timer)
      version++
      result.value = null
      error.value = ''
    } else if (metadata) {
      const savedHtml = metadata.has_page ? result.value?.html : undefined
      result.value = { ...metadata, ...(savedHtml ? { html: savedHtml } : {}) }
    } else if (!result.value?.html) {
      result.value = null
    }
    if (metadata?.has_page || metadata?.state === 'running') void refresh()
  },
  { immediate: true }
)
function view() {
  open.value = true
  if (!source.value && !loading.value) void refresh()
}
function close() { open.value = false }
async function start() {
  if (!props.canTest || blocked.value || sending.value) return
  sending.value = true; error.value = ''
  clearTimeout(timer); version++
  try {
    await api.startDowngradeTest(props.channelId, props.account.id)
    result.value = { ...current.value, state: 'running', started_at: new Date().toISOString(), generated_at: current.value?.generated_at || null, has_page: !!current.value?.has_page }
    emit('changed')
    await refresh()
  } catch (e) { if (!disposed) error.value = message(e) }
  finally { sending.value = false }
}
async function fullscreen() {
  try { if (document.fullscreenElement) await document.exitFullscreen(); else await preview.value?.requestFullscreen() }
  catch { error.value = '当前浏览器不支持全屏预览' }
}
onUnmounted(() => { disposed = true; version++; clearTimeout(timer) })
</script>

<template>
  <section class="downgrade-test" :class="{ compact }">
    <div class="test-details">
      <div class="test-summary">
        <div><h3>{{ account.name }} · 降智测试 <span :data-state="current?.state">{{ status }}</span></h3><p>{{ current?.has_page ? `生成于 ${stamp(current.generated_at)}` : '暂无测试页面' }}</p></div>
        <div class="test-actions">
          <button v-if="canTest" class="btn btn-primary" :disabled="sending || blocked" @click="start"><Icon name="play" size="sm" />{{ sending || running ? '测试中…' : '开始降智测试' }}</button>
        </div>
      </div>
      <p v-if="!compact" class="test-model">gpt-6-astra · 高推理 · 中转额度计费</p>
      <p v-if="current?.error" class="test-warning">{{ current.error }}</p>
    </div>
    <div class="inline-preview">
      <div class="inline-preview-toolbar">
        <span>测试页面预览</span>
        <button v-if="current?.has_page || source" class="btn btn-secondary" @click="view"><Icon name="eye" size="sm" />放大预览</button>
      </div>
      <p v-if="error" role="alert" class="test-warning">{{ error }} <button class="btn btn-secondary" :disabled="loading" @click="refresh">重试加载</button></p>
      <p v-if="running || sending" role="status" class="test-progress">正在生成新页面{{ source ? '，当前显示上次生成结果' : '' }}…</p>
      <iframe v-if="inlineSource" class="inline-preview-frame" title="降智测试页面预览" sandbox="allow-scripts" referrerpolicy="no-referrer" :srcdoc="inlineSource" />
      <div v-else class="inline-preview-empty">{{ loading ? '加载中…' : running || sending ? '服务器正在生成本轮页面…' : '暂无测试页面' }}</div>
    </div>
    <BaseDialog :show="open" :title="`${account.name} · 降智测试`" width="full" @close="close">
      <div ref="preview" class="test-preview">
        <div class="preview-toolbar"><div><strong>gpt-6-astra</strong><span>高推理</span><span>{{ status }}</span><time v-if="current?.generated_at">{{ stamp(current.generated_at) }}</time></div><button class="btn btn-secondary" aria-label="全屏预览" title="全屏预览" @click="fullscreen"><Icon name="arrowsExpand" size="sm" /></button></div>
        <p v-if="error" role="alert" class="test-warning">{{ error }} <button class="btn btn-secondary" :disabled="loading" @click="refresh">重试加载</button></p>
        <p v-if="current?.error" role="alert" class="test-warning">{{ current.error }}</p>
        <p v-if="running || sending" role="status" class="test-progress">正在生成新页面{{ source ? '，下方为当前已保存页面' : '' }}…</p>
        <iframe v-if="source" class="preview-frame" title="降智测试页面" sandbox="allow-scripts" referrerpolicy="no-referrer" :srcdoc="source" />
        <div v-else class="preview-empty">{{ loading ? '加载中…' : running || sending ? '等待测试结果…' : '暂无测试页面' }}</div>
      </div>
    </BaseDialog>
  </section>
</template>

<style scoped>
.downgrade-test{container-type:inline-size;border-top:1px solid var(--ex-border,#e5e7eb);padding:20px 0}.downgrade-test.compact{padding:14px 0 4px;margin-top:6px}.test-details{min-width:0}.test-summary,.test-actions,.preview-toolbar,.preview-toolbar>div{display:flex;align-items:center;gap:10px;flex-wrap:wrap}.test-summary{justify-content:space-between}.test-summary h3{font-size:14px;font-weight:600;margin:0}.test-summary h3 span{font-size:11px;font-weight:400;margin-left:8px;color:#6b7280}.test-summary h3 span[data-state=running]{color:#a16207}.test-summary p,.test-model{font-size:12px;color:#6b7280;margin-top:6px}.test-actions .btn{font-size:12px;white-space:nowrap}.test-warning,.test-progress{padding:8px 10px;background:#fffbeb;color:#854d0e;border:1px solid #fde68a;border-radius:6px;margin:8px 0;font-size:12px}.test-preview{display:flex;flex-direction:column;min-width:0;background:white;color:#1f2937}.preview-toolbar{justify-content:space-between;padding:0 0 12px}.preview-toolbar>div{font-size:12px}.preview-toolbar span,.preview-toolbar time{color:#6b7280}.preview-frame{border:1px solid #e5e7eb;width:100%;height:65dvh;min-height:280px;background:white}.preview-empty{display:grid;place-items:center;min-height:45dvh;color:#6b7280}.test-preview:fullscreen{padding:16px;height:100%;width:100%}.test-preview:fullscreen .preview-frame{flex:1;min-height:0}.test-summary>div{min-width:0}.test-summary p{overflow-wrap:anywhere}.inline-preview{width:min(100%,340px);margin:10px 0 0 auto;min-width:0}.inline-preview-toolbar{display:flex;align-items:center;justify-content:space-between;gap:10px;margin-bottom:6px;color:#6b7280;font-size:11px}.inline-preview-toolbar .btn{font-size:11px;white-space:nowrap;padding:7px 10px}.inline-preview-frame,.inline-preview-empty{display:block;width:100%;aspect-ratio:4/3;border:1px solid #e5e7eb;border-radius:6px;background:#fff}.inline-preview-empty{display:grid;place-items:center;color:#6b7280;background:#fafafa}
@container (min-width:540px){.downgrade-test.compact{display:grid;grid-template-columns:minmax(0,1fr) minmax(280px,340px);align-items:start;gap:16px}.compact .inline-preview{width:100%;margin-top:0}.compact .test-summary{align-items:flex-start}.compact .test-actions{width:100%;margin-top:8px}}
@media(max-width:650px){.test-actions{width:100%}.preview-frame{height:62dvh}.preview-toolbar{gap:8px}.test-preview:fullscreen{padding:8px}.inline-preview{width:100%}}
</style>
