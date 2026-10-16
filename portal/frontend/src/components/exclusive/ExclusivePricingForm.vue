<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import type { ChannelModelPrice } from '@/api/exclusiveChannels'
const props = defineProps<{ modelValue: ChannelModelPrice[]; imagePrice: number | null; imageRate?: number | null; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [ChannelModelPrice[]]; 'update:imagePrice': [number | null]; 'update:imageRate': [number | null] }>()
const fields = [{ key: 'input_price', label: '输入' }, { key: 'output_price', label: '输出' }, { key: 'cache_read_price', label: '缓存读取' }] as const
const displayPrice = (v: number | null) => v == null ? '' : Number((v * 1e6).toPrecision(12))
function change(index: number, field: keyof ChannelModelPrice, event: Event) {
  const input = event.target as HTMLInputElement
  const value = field === 'model' ? input.value : input.value === '' ? null : Number(input.value) / 1e6
  emit('update:modelValue', props.modelValue.map((row, i) => i === index ? { ...row, [field]: value } : row))
}
function add() { emit('update:modelValue', [...props.modelValue, { model: '', input_price: null, output_price: null, cache_read_price: null }]) }
</script>
<template>
  <fieldset class="pricing-editor" :disabled="disabled">
    <legend>OpenAI 定价</legend>
    <section class="image-pricing">
      <label for="exclusive-image-price">图片单价 <span>元 / 张</span></label>
      <div class="image-price-input"><input id="exclusive-image-price" type="number" min="0" step="any" :value="imagePrice ?? ''" placeholder="默认 0.05" aria-label="图片单价（元/张）" @input="emit('update:imagePrice', ($event.target as HTMLInputElement).value === '' ? null : Number(($event.target as HTMLInputElement).value))" /><span>× 图片倍率</span><input type="number" min="0.001" step="0.001" :value="imageRate ?? ''" placeholder="默认 1" aria-label="图片倍率" @input="emit('update:imageRate', ($event.target as HTMLInputElement).value === '' ? null : Number(($event.target as HTMLInputElement).value))" /></div>
      <p>默认 ¥0.05 / 张 · 图片倍率默认 1 · 所有尺寸同价</p>
    </section>
    <div class="pricing-heading"><h4>文本模型 <span>元 / 百万 Token</span></h4><button type="button" class="btn btn-secondary" :disabled="disabled || modelValue.length >= 100" aria-label="添加模型定价" title="添加模型定价" @click="add"><Icon name="plus" size="sm" /></button></div>
    <p class="pricing-note">空白：默认价格 · 0：免费 · 最终费用 × 通道倍率</p>
    <p v-if="!modelValue.length" class="pricing-empty">全部文本模型使用默认价格</p>
    <div v-for="(entry, index) in modelValue" :key="index" class="model-price-row">
      <div class="model-name"><input :value="entry.model" required maxlength="128" pattern="[A-Za-z0-9][A-Za-z0-9._-]{0,127}" :aria-label="`模型 ${index + 1} 名称`" placeholder="例如 gpt-6-astra" @input="change(index, 'model', $event)" /><button type="button" class="btn btn-secondary" :aria-label="`删除模型 ${index + 1} 定价`" title="恢复此模型默认价格" @click="emit('update:modelValue', modelValue.filter((_, i) => i !== index))"><Icon name="trash" size="sm" /></button></div>
      <div class="model-price-inputs"><label v-for="field in fields" :key="field.key"><span>{{ field.label }}</span><input type="number" min="0" step="any" :value="displayPrice(entry[field.key])" :aria-label="`模型 ${index + 1} ${field.label}价格`" placeholder="默认" @input="change(index, field.key, $event)" /></label></div>
    </div>
  </fieldset>
</template>
<style scoped>
.pricing-editor { min-width:0; padding:20px 0; margin-bottom:20px; border-top:1px solid var(--ex-border,#e3e7ea); border-bottom:1px solid var(--ex-border,#e3e7ea); }.pricing-editor legend { font-weight:600; padding-right:12px; }.pricing-editor:disabled { opacity:.65; }
.image-pricing>label,.pricing-heading h4 { font-size:13px; font-weight:600; }.image-pricing span,.pricing-heading h4 span { color:var(--ex-muted,#697582); font-size:11px; font-weight:400; margin-left:6px; }
.image-price-input { display:flex; align-items:center; gap:8px; margin-top:10px; }.image-price-input input { width:130px; min-width:0; }.image-price-input span { flex-shrink:0; }.image-pricing p,.pricing-note,.pricing-empty { color:var(--ex-muted,#697582); font-size:12px; margin-top:8px; line-height:1.7; }
.pricing-heading { display:flex; justify-content:space-between; align-items:center; margin-top:24px; gap:10px; }.pricing-empty { padding:12px 0; }.model-price-row { padding:16px 0; border-top:1px solid var(--ex-border,#e3e7ea); margin-top:12px; }.model-name { display:flex; align-items:center; gap:10px; }.model-name>input { flex:1; width:0; }
.model-price-inputs { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:12px; margin-top:12px; }.model-price-inputs label { display:flex; flex-direction:column; gap:6px; font-size:12px; color:var(--ex-muted,#697582); }
input { min-width:0; padding:9px 10px; border:1px solid var(--ex-border,#e3e7ea); border-radius:6px; background:transparent; color:inherit; font-size:13px; }input:focus-visible { outline:2px solid #3980cf; outline-offset:2px; }
@media(max-width:650px){input { font-size:16px; }.model-price-inputs { gap:8px; }.pricing-heading h4 span { display:block; margin:4px 0 0; }}
</style>
