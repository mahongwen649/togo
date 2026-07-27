import test from 'node:test'
import assert from 'node:assert/strict'
import { unwrap } from '../src/site.js'

test('unwraps portal API envelopes', () => {
  assert.deepEqual(unwrap({ code: 0, data: { items: [1] } }), { items: [1] })
  assert.throws(() => unwrap({ code: 500, message: 'failed' }), /failed/)
})
