<template>
  <AppLayout>
    <main class="developer-page">
      <div class="developer-shell">
        <header class="developer-hero">
          <span class="eyebrow">{{ t('developers.eyebrow') }}</span>
          <h1>{{ t('developers.title') }}</h1>
          <p>{{ t('developers.description') }}</p>
          <div class="endpoint"><span>BASE URL</span><code>{{ baseUrl }}</code></div>
        </header>

        <section class="quick-start" :aria-labelledby="quickStartId">
          <div class="section-heading">
            <h2 :id="quickStartId">{{ t('developers.quickStart') }}</h2>
            <span></span>
          </div>
          <div class="quick-grid">
            <article v-for="step in quickSteps" :key="step.number" class="quick-step">
              <div><span class="step-number">{{ step.number }}</span><h3>{{ step.title }}</h3></div>
              <code>{{ step.code }}</code>
              <RouterLink v-if="step.link" :to="step.link">{{ t('developers.openKeys') }} →</RouterLink>
            </article>
          </div>
        </section>

        <section :aria-labelledby="integrationId">
          <div class="section-heading">
            <h2 :id="integrationId">{{ t('developers.integrations') }}</h2>
            <span></span>
          </div>
          <div class="integration-grid">
            <article v-for="card in integrationCards" :key="card.key" class="integration-card">
              <div class="card-top">
                <span class="card-icon"><Icon :name="card.icon" size="lg" /></span>
                <span class="card-tag">{{ card.tag }}</span>
              </div>
              <h3>{{ card.title }}</h3>
              <p>{{ card.body }}</p>
              <CodeBlock :code="card.code" compact />
            </article>
          </div>
        </section>

        <section :aria-labelledby="examplesId">
          <div class="section-heading">
            <h2 :id="examplesId">{{ t('developers.examples') }}</h2>
            <span></span>
          </div>
          <div class="example-grid">
            <article v-for="example in examples" :key="example.key" class="example-card">
              <h3>{{ example.title }}</h3>
              <CodeBlock :code="example.code" />
            </article>
          </div>
        </section>

        <p class="developer-footer">{{ t('developers.footer') }}</p>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const baseUrl = 'https://api.togoapi.com/v1'
const quickStartId = 'developer-quick-start'
const integrationId = 'developer-integrations'
const examplesId = 'developer-examples'

const pythonCode = `from openai import OpenAI

client = OpenAI(
    api_key="YOUR_API_KEY",
    base_url="${baseUrl}"
)

response = client.chat.completions.create(
    model="gpt-5.1-codex",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)`

const nodeCode = `import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.TOGO_API_KEY,
  baseURL: "${baseUrl}",
});

const response = await client.chat.completions.create({
  model: "gpt-5.1-codex",
  messages: [{ role: "user", content: "Hello" }],
});
console.log(response.choices[0].message.content);`

const curlCode = `curl ${baseUrl}/chat/completions \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-5.1-codex","messages":[{"role":"user","content":"Hello"}]}'`

const envCode = `OPENAI_API_KEY=YOUR_API_KEY
OPENAI_BASE_URL=${baseUrl}`

const quickSteps = computed(() => [
  { number: '1', title: t('developers.steps.createKey'), code: 'sk-togo-••••••••', link: '/keys' },
  { number: '2', title: t('developers.steps.installSdk'), code: 'npm install openai' },
  { number: '3', title: t('developers.steps.makeRequest'), code: 'client.chat.completions.create(...)' },
])

const integrationCards = computed(() => [
  { key: 'openai', icon: 'sparkles' as const, tag: t('developers.cards.openai.tag'), title: t('developers.cards.openai.title'), body: t('developers.cards.openai.body'), code: `base_url="${baseUrl}"` },
  { key: 'curl', icon: 'server' as const, tag: t('developers.cards.curl.tag'), title: t('developers.cards.curl.title'), body: t('developers.cards.curl.body'), code: `POST ${baseUrl}/chat/completions` },
  { key: 'agents', icon: 'terminal' as const, tag: t('developers.cards.agents.tag'), title: t('developers.cards.agents.title'), body: t('developers.cards.agents.body'), code: envCode },
])

const examples = computed(() => [
  { key: 'python', title: t('developers.exampleTitles.python'), code: pythonCode },
  { key: 'node', title: t('developers.exampleTitles.node'), code: nodeCode },
  { key: 'curl', title: t('developers.exampleTitles.curl'), code: curlCode },
  { key: 'env', title: t('developers.exampleTitles.env'), code: envCode },
])

const CodeBlock = defineComponent({
  props: { code: { type: String, required: true }, compact: Boolean },
  setup(props) {
    const state = ref<'idle' | 'copied' | 'failed'>('idle')
    let resetTimer: ReturnType<typeof setTimeout> | undefined
    async function copy() {
      try {
        await navigator.clipboard.writeText(props.code)
        state.value = 'copied'
      } catch {
        state.value = 'failed'
      }
      clearTimeout(resetTimer)
      resetTimer = setTimeout(() => { state.value = 'idle' }, 1600)
    }
    return () => h('div', { class: ['code-block', props.compact && 'code-block-compact'] }, [
      h('pre', [h('code', props.code)]),
      h('button', {
        type: 'button',
        class: 'copy-button',
        title: t('developers.copy'),
        'aria-label': t('developers.copy'),
        onClick: copy,
      }, [
        h(Icon, { name: state.value === 'copied' ? 'check' : 'copy', size: 'sm' }),
        h('span', { class: 'sr-only', 'aria-live': 'polite' }, state.value === 'copied' ? t('developers.copied') : state.value === 'failed' ? t('developers.copyFailed') : ''),
      ]),
    ])
  },
})
</script>

<style scoped>
.developer-page { min-height: 100%; overflow-y: auto; background: #f8fafc; color: #0f172a; }
.dark .developer-page { background: #020617; color: #f8fafc; }
.developer-shell { width: min(1120px, 100%); margin: 0 auto; padding: 48px 24px 36px; display: flex; flex-direction: column; gap: 44px; }
.developer-hero { display: flex; flex-direction: column; align-items: center; text-align: center; gap: 14px; }
.eyebrow { border: 1px solid #cbd5e1; background: #fff; color: #475569; padding: 5px 10px; border-radius: 999px; font-size: 11px; font-weight: 700; text-transform: uppercase; }
.dark .eyebrow { border-color: #334155; background: #0f172a; color: #94a3b8; }
.developer-hero h1 { font-size: clamp(32px, 5vw, 52px); line-height: 1.05; font-weight: 750; }
.developer-hero p { max-width: 680px; color: #64748b; font-size: 17px; line-height: 1.65; }
.dark .developer-hero p { color: #94a3b8; }
.endpoint { display: flex; align-items: center; gap: 10px; margin-top: 4px; padding: 8px 12px; border: 1px solid #bae6fd; background: #f0f9ff; border-radius: 8px; }
.endpoint span { font-size: 10px; font-weight: 800; color: #0369a1; }
.endpoint code { font-size: 12px; color: #0c4a6e; }
.dark .endpoint { border-color: #115e59; background: #0b2f2b; }
.dark .endpoint span, .dark .endpoint code { color: #67e8f9; }
.section-heading { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
.section-heading h2 { flex: none; font-size: 12px; font-weight: 800; text-transform: uppercase; color: #64748b; }
.section-heading span { height: 1px; flex: 1; background: #e2e8f0; }
.dark .section-heading h2 { color: #94a3b8; }
.dark .section-heading span { background: #1e293b; }
.quick-start { padding: 24px; border: 1px solid #e2e8f0; background: #fff; border-radius: 8px; box-shadow: 0 1px 3px rgb(15 23 42 / .05); }
.dark .quick-start { border-color: #1e293b; background: #0f172a; }
.quick-grid, .integration-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.quick-step { min-width: 0; padding: 16px; border: 1px solid #e2e8f0; background: #f8fafc; border-radius: 8px; }
.dark .quick-step { border-color: #334155; background: #020617; }
.quick-step > div { display: flex; align-items: center; gap: 9px; margin-bottom: 10px; }
.step-number { width: 24px; height: 24px; display: grid; place-items: center; flex: none; border-radius: 50%; background: #0f172a; color: #fff; font-size: 12px; font-weight: 800; }
.dark .step-number { background: #f8fafc; color: #020617; }
.quick-step h3 { font-size: 14px; font-weight: 700; }
.quick-step code { display: block; overflow: hidden; text-overflow: ellipsis; color: #0f766e; font-size: 12px; white-space: nowrap; }
.quick-step a { display: inline-block; margin-top: 9px; color: #0284c7; font-size: 12px; font-weight: 700; }
.integration-card { min-width: 0; padding: 22px; display: flex; flex-direction: column; gap: 12px; border: 1px solid #e2e8f0; background: #fff; border-radius: 8px; }
.integration-card:hover { border-color: #7dd3fc; box-shadow: 0 8px 24px rgb(15 23 42 / .07); }
.dark .integration-card { border-color: #1e293b; background: #0f172a; }
.dark .integration-card:hover { border-color: #155e75; }
.card-top { display: flex; align-items: center; justify-content: space-between; }
.card-icon { width: 40px; height: 40px; display: grid; place-items: center; border-radius: 8px; background: #f0fdfa; color: #0f766e; }
.dark .card-icon { background: #042f2e; color: #5eead4; }
.card-tag { font-size: 10px; font-weight: 800; color: #64748b; }
.integration-card h3 { font-size: 18px; font-weight: 750; }
.integration-card p { min-height: 62px; color: #64748b; font-size: 13px; line-height: 1.6; }
.dark .integration-card p { color: #94a3b8; }
.example-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
.example-card { min-width: 0; padding: 18px; border: 1px solid #e2e8f0; background: #fff; border-radius: 8px; }
.dark .example-card { border-color: #1e293b; background: #0f172a; }
.example-card h3 { margin-bottom: 10px; font-size: 13px; font-weight: 750; }
:deep(.code-block) { position: relative; min-width: 0; border: 1px solid #1e293b; background: #020617; border-radius: 8px; overflow: hidden; }
:deep(.code-block pre) { min-height: 148px; overflow: auto; padding: 16px 42px 16px 16px; color: #a5f3fc; font-size: 11px; line-height: 1.65; white-space: pre; }
:deep(.code-block-compact pre) { min-height: 62px; max-height: 86px; }
:deep(.copy-button) { position: absolute; top: 8px; right: 8px; width: 30px; height: 30px; display: grid; place-items: center; color: #94a3b8; border: 1px solid #334155; background: #0f172a; border-radius: 6px; }
:deep(.copy-button:hover) { color: #fff; border-color: #475569; }
.developer-footer { text-align: center; color: #94a3b8; font-size: 12px; }
@media (max-width: 860px) { .quick-grid, .integration-grid { grid-template-columns: 1fr; } .integration-card p { min-height: auto; } }
@media (max-width: 680px) { .developer-shell { padding: 32px 16px 28px; gap: 34px; } .quick-start { padding: 16px; } .example-grid { grid-template-columns: 1fr; } .endpoint { max-width: 100%; } .endpoint code { overflow: hidden; text-overflow: ellipsis; } }
</style>
