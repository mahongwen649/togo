<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { pagePreviewDocument } from '@/components/exclusive/pagePreview'
import { renderAccountCheckMarkdown } from '@/utils/accountCheckMarkdown'
import type { CheckAccount, AccountCheckResult } from '@/api/admin/accountChecks'
import 'katex/dist/katex.min.css'

const props = defineProps<{ account: CheckAccount; prompt: string; model: string; state: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'; result: AccountCheckResult | null; error: string; elapsed: number; embedded?: boolean; cancelling?: boolean }>()
defineEmits<{ cancel: [] }>()
const tab = ref<'preview' | 'text'>('text'), version = ref(0), preview = ref<HTMLElement | null>(null), previewError = ref('')
const content = ref<HTMLElement | null>(null)
const source = computed(() => props.result?.html && !props.error ? pagePreviewDocument(props.result.html, 'fit') : '')
const upstreamError = computed(() => {
  if (/websocket closed before terminal event.*1008/i.test(props.error)) return { title: '上游连接提前关闭（1008）', help: '供应商未完成回答就关闭了连接，本次检查未完成，不能据此判断账号是否降智。重复出现时需供应商排查其连接策略或超时限制。' }
  if (/API returned 50[234]:/i.test(props.error)) return { title: '上游服务暂时不可用', help: props.result?.attempts === 2 ? '已自动重试 1 次，上游仍未正常返回。请稍后重新检查，持续失败需联系供应商。' : '供应商返回服务错误，请稍后重新检查，持续失败需联系供应商。' }
  return null
})
const partialCode = computed(() => !!props.error && /<\s*(?:!doctype|html|svg|body|style|script)\b|```(?:html|svg|css)/i.test(props.result?.text || ''))
const renderedText = computed(() => renderAccountCheckMarkdown(props.result?.text || ''))
const status = computed(() => ({ queued: '等待检查', running: '检查中', succeeded: '已完成', failed: '检查失败', cancelled: '已停止' })[props.state])
watch(() => props.result, result => { tab.value = result?.html ? 'preview' : 'text'; version.value++; previewError.value = '' }, { immediate: true })
async function fullscreen() {
  try { await (tab.value === 'preview' && source.value ? preview.value : content.value)?.requestFullscreen() }
  catch { previewError.value = '当前浏览器不支持全屏预览' }
}
</script>

<template>
  <article class="card result-card overflow-hidden" :class="{ 'embedded-result': embedded }" :data-result-account="account.id" :aria-label="`账号 ${account.id} 的检查结果`" :aria-busy="state === 'running'">
    <div class="result-header border-b border-gray-200 dark:border-dark-500">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h3 class="min-w-0 break-words text-sm font-semibold">{{ embedded ? '检查结果' : `#${account.id} · ${account.name}` }}</h3>
        <div class="flex items-center gap-2 text-xs"><span class="result-status" :class="state === 'failed' ? 'text-red-600' : state === 'succeeded' ? 'text-green-600' : 'text-gray-500'">{{ status }}<span v-if="elapsed"> · {{ elapsed }} 秒</span></span><button v-if="state === 'queued' || state === 'running'" type="button" class="btn btn-secondary btn-sm" :disabled="cancelling" @click="$emit('cancel')">{{ state === 'queued' ? '取消' : cancelling ? '正在终止…' : '终止检查' }}</button></div>
      </div>
      <p v-if="!embedded && (result?.model || model)" class="mt-1 text-xs text-gray-500">模型：{{ result?.model || model }}</p>
      <details class="mt-2 text-xs text-gray-500"><summary class="cursor-pointer">本次题目</summary><p class="mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap break-words">{{ prompt }}</p></details>
    </div>
    <div v-if="upstreamError" role="alert" class="upstream-error"><strong>{{ upstreamError.title }}</strong><p>{{ upstreamError.help }}</p><details><summary>技术详情</summary><p>{{ error }}</p></details></div>
    <p v-else-if="error" role="alert" class="m-3 break-words rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
    <details v-if="partialCode" class="partial-output"><summary>查看未完成的原始输出（未渲染）</summary><pre>{{ result?.text }}</pre></details>
    <div v-else-if="result?.text || source" ref="content" class="result-body">
      <div v-if="source" class="result-actions mb-3 flex flex-wrap items-center gap-2">
        <button type="button" class="btn btn-sm" :class="tab === 'preview' ? 'btn-primary' : 'btn-secondary'" @click="tab = 'preview'">页面 / 动画预览</button>
        <button type="button" class="btn btn-sm" :class="tab === 'text' ? 'btn-primary' : 'btn-secondary'" @click="tab = 'text'">原始输出</button>
        <button v-if="tab === 'preview'" type="button" class="btn btn-secondary btn-sm" @click="version++">重新播放</button>
        <button v-if="tab === 'preview'" type="button" class="btn btn-secondary btn-sm" @click="fullscreen">全屏预览</button>
      </div>
      <button v-if="embedded && (!source || tab === 'text')" type="button" class="expand-text" @click="fullscreen">放大查看</button>
      <p v-if="previewError" role="alert" class="mb-2 text-sm text-amber-600">{{ previewError }}</p>
      <div v-if="source && tab === 'preview'" ref="preview" class="check-preview">
        <iframe :key="version" :title="`账号 ${account.id} 的页面与动画预览`" sandbox="allow-scripts" referrerpolicy="no-referrer" :srcdoc="source" />
        <p class="preview-caption">自适应完整预览 · 支持全屏查看</p>
      </div>
      <pre v-else-if="source" class="result-text whitespace-pre-wrap break-words font-mono text-xs leading-6 text-gray-800 dark:text-gray-200" tabindex="0" aria-label="原始输出">{{ result?.text }}</pre>
      <div v-else class="result-text check-markdown text-sm leading-7 text-gray-800 dark:text-gray-200" tabindex="0" aria-label="答案文案" v-html="renderedText" />
    </div>
    <div v-else-if="!error" class="grid min-h-32 place-items-center p-5 text-sm text-gray-500">{{ state === 'queued' ? '正在排队，轮到此账号后自动开始。' : state === 'running' ? '模型正在生成，请稍候…' : '本次未获得结果。' }}</div>
  </article>
</template>

<style scoped>
.result-card { min-width: 0; border-radius: 12px; box-shadow: 0 2px 8px #163e3104; }
.upstream-error { margin: 8px 0; padding: 10px; border-radius: 7px; background: #fef2f2; color: #b91c1c; font-size: 11px; line-height: 1.7; overflow-wrap: anywhere; }
.upstream-error details { margin-top: 6px; font-size: 10px; }
.upstream-error summary, .partial-output summary { cursor: pointer; }
.partial-output { margin-top: 8px; color: var(--muted, #667e77); font-size: 11px; }
.partial-output pre { margin-top: 6px; max-height: 160px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; padding: 8px; background: var(--subtle, #f7faf9); }
:global(.dark) .upstream-error { background: #451d25; color: #fca5a5; }
.result-header { padding: 13px 16px; }
.result-body { padding: 14px; }
.result-status { white-space: nowrap; padding: 3px 7px; border-radius: 5px; background: #94a3b810; }
.embedded-result { margin-top: 10px; border: 0; border-top: 1px solid var(--line, #e2e9e8); border-radius: 0; box-shadow: none; background: transparent; }
.embedded-result .result-header { padding: 9px 0 7px; border: 0; }
.embedded-result .result-header h3 { font-size: 11px; color: var(--muted, #667e77); font-weight: 500; }
.embedded-result .result-header details { font-size: 10px; margin-top: 4px; }
.embedded-result .result-header details p { max-height: 64px; }
.embedded-result .result-status { font-size: 10px; }
.embedded-result .result-body { position: relative; padding: 0; }
.embedded-result .result-actions { gap: 4px; margin-bottom: 7px; }
.embedded-result .result-actions .btn { font-size: 10px; padding: 4px 6px; }
.embedded-result .result-text { height: 160px; max-height: 160px; padding: 24px 10px 10px; background: var(--subtle, #f7faf9); border: 1px solid var(--line, #e2e9e8); border-radius: 7px; font-size: 12px; line-height: 1.8; }
.embedded-result .check-preview:not(:fullscreen) iframe { height: 160px; aspect-ratio: auto; }
.embedded-result .preview-caption { display: none; }
.expand-text { position: absolute; top: 4px; right: 8px; z-index: 1; color: var(--accent, #0d9488); background: var(--subtle, #f7faf9); padding: 1px 4px; border-radius: 3px; font-size: 10px; }
.result-body:fullscreen { display: flex; flex-direction: column; padding: 24px; background: var(--surface, white); overflow: auto; }
.result-body:fullscreen .result-text { flex: 1; height: auto; max-height: none; font-size: 16px; }
.result-body:fullscreen .expand-text { display: none; }
.result-actions .btn { font-size: 11px; padding: 5px 9px; }
.result-text { max-height: 420px; overflow: auto; overflow-wrap: anywhere; scrollbar-width: thin; }
.preview-caption { margin-top: 8px; font-size: 10px; text-align: right; color: #74867f; }
.check-preview { width: min(100%, 900px); margin: 0 auto; }
.check-preview iframe { display: block; width: 100%; aspect-ratio: 16 / 9; min-height: 0; border: 1px solid #e5e7eb; border-radius: 8px; background: white; }
.check-preview:fullscreen { width: 100%; height: 100%; padding: 16px; display: flex; flex-direction: column; background: white; }
.check-preview:fullscreen iframe { flex: 1; height: 100%; aspect-ratio: auto; }
.check-markdown :deep(p) { margin: 0 0 0.9rem; }
.check-markdown :deep(p:last-child) { margin-bottom: 0; }
.check-markdown :deep(ul), .check-markdown :deep(ol) { margin: 0.6rem 0 0.9rem; padding-left: 1.5rem; }
.check-markdown :deep(ul) { list-style: disc; }
.check-markdown :deep(ol) { list-style: decimal; }
.check-markdown :deep(strong) { font-weight: 700; }
.check-markdown :deep(code) { border-radius: 4px; background: #f3f4f6; padding: 0.1rem 0.3rem; font-family: ui-monospace, monospace; }
.check-markdown :deep(pre) { margin: 0.75rem 0; overflow-x: auto; border-radius: 8px; background: #111827; padding: 0.9rem; color: #f9fafb; }
.check-markdown :deep(pre code) { background: transparent; padding: 0; }
.check-markdown :deep(.katex-display) { margin: 1rem 0; overflow-x: auto; overflow-y: hidden; }
:global(.dark) .check-markdown :deep(code) { background: #263244; }
</style>
