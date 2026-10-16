import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ExclusivePricingForm from './ExclusivePricingForm.vue'
import ExclusivePricingSummary from './ExclusivePricingSummary.vue'
import type { ExclusiveChannel } from '@/api/exclusiveChannels'

describe('Exclusive pricing editor', () => {
  it('converts per-million input once, preserving empty and explicit zero', async () => {
    const w = mount(ExclusivePricingForm, { props: { modelValue: [{ model: 'gpt-6-astra', input_price: 0.0000025, output_price: null, cache_read_price: 0 }], imagePrice: 0.05 } })
    expect((w.get('[aria-label="模型 1 输入价格"]').element as HTMLInputElement).value).toBe('2.5')
    expect((w.get('[aria-label="模型 1 输出价格"]').element as HTMLInputElement).value).toBe('')
    expect((w.get('[aria-label="模型 1 缓存读取价格"]').element as HTMLInputElement).value).toBe('0')
    await w.get('[aria-label="模型 1 输入价格"]').setValue('4')
    expect(w.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([{ model: 'gpt-6-astra', input_price: 0.000004, output_price: null, cache_read_price: 0 }])
    await w.get('[aria-label="模型 1 输入价格"]').setValue('')
    expect(w.emitted('update:modelValue')?.at(-1)?.[0]).toEqual([{ model: 'gpt-6-astra', input_price: null, output_price: null, cache_read_price: 0 }])
    w.unmount()
  })

  it('keeps image prices per image and restores the default when cleared', async () => {
    const w = mount(ExclusivePricingForm, { props: { modelValue: [], imagePrice: 0.05 } })
    const input = w.get('[aria-label="图片单价（元/张）"]')
    await input.setValue('0.08')
    expect(w.emitted('update:imagePrice')?.at(-1)).toEqual([0.08])
    await input.setValue('')
    expect(w.emitted('update:imagePrice')?.at(-1)).toEqual([null])
    await w.get('[aria-label="图片倍率"]').setValue('1.5')
    expect(w.emitted('update:imageRate')?.at(-1)).toEqual([1.5])
    await w.get('[aria-label="图片倍率"]').setValue('')
    expect(w.emitted('update:imageRate')?.at(-1)).toEqual([null])
    expect(w.text()).toContain('默认 ¥0.05 / 张')
    w.unmount()
  })

  it('adds and removes model overrides without changing existing entries', async () => {
    const w = mount(ExclusivePricingForm, { props: { modelValue: [], imagePrice: null } })
    await w.get('[aria-label="添加模型定价"]').trigger('click')
    const rows = [{ model: '', input_price: null, output_price: null, cache_read_price: null }]
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([rows])
    await w.setProps({ modelValue: rows })
    await w.get('[aria-label="删除模型 1 定价"]').trigger('click')
    expect(w.emitted('update:modelValue')?.at(-1)).toEqual([[]])
    await w.setProps({ disabled: true })
    expect(w.get('fieldset').attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('shows the price quote and does not advertise unsynchronized prices as live', async () => {
    const channel = { image_price: 0.05, rate_multiplier: 2, pricing_synced: true, model_pricing: [{ model: 'gpt-6-astra', input_price: 0.0000025, output_price: null, cache_read_price: 0 }] } as ExclusiveChannel
    const w = mount(ExclusivePricingSummary, { props: { channel } })
    expect(w.text()).toContain('¥0.05')
    expect(w.get('.image-price-line').text()).toContain('× 1')
    expect(w.text()).toContain('× 2')
    expect(w.text()).toContain('2.5')
    expect(w.text()).toContain('默认')
    await w.setProps({ channel: { ...channel, pricing_synced: false } })
    expect(w.text()).toContain('新价格待同步')
    expect(w.text()).not.toContain('¥0.05')
    w.unmount()
  })
})
