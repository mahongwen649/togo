import { afterEach, describe, expect, it, vi } from 'vitest'
import { runAccountCheck } from '../accountChecks'

afterEach(() => vi.unstubAllGlobals())

describe('account check result stream', () => {
  it('ignores heartbeat comments and decodes UTF-8 across arbitrary chunks, including the final line', async () => {
    const result = { text: '中文答案', html: '', model: 'model-a' }
    const bytes = new TextEncoder().encode(`: checking\n\ndata: ${JSON.stringify(result)}`)
    const stream = new ReadableStream({ start(controller) { for (const byte of bytes) controller.enqueue(new Uint8Array([byte])); controller.close() } })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(stream)))
    await expect(runAccountCheck(42, { prompt: '问题', model_id: '' }, new AbortController().signal)).resolves.toEqual(result)
  })
  it('rejects interrupted streams and surfaces server authorization errors', async () => {
    const fetch = vi.fn().mockResolvedValueOnce(new Response(': checking\n\n')).mockResolvedValueOnce(new Response(JSON.stringify({ message: 'Admin access required' }), { status: 403 }))
    vi.stubGlobal('fetch', fetch)
    await expect(runAccountCheck(42, { prompt: '问题', model_id: '' }, new AbortController().signal)).rejects.toThrow('检查响应中断')
    await expect(runAccountCheck(42, { prompt: '问题', model_id: '' }, new AbortController().signal)).rejects.toThrow('Admin access required')
  })
})
