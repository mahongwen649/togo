import test from 'node:test'
import assert from 'node:assert/strict'
import { loadConfig } from '../src/config.js'

test('loads group whitelist and commands', () => {
  const config = loadConfig({
    ONEBOT_WS_URL: 'ws://127.0.0.1:3001',
    ONEBOT_TOKEN: 'secret',
    SITE_BASE_URL: 'https://example.com/',
    SITE_IDENTIFIER: 'bot',
    SITE_PASSWORD: 'password',
    ALLOWED_GROUP_IDS: '123, 456',
    COMMANDS: '状态检查,站点状态',
  })
  assert.equal(config.siteBaseUrl, 'https://example.com')
  assert.deepEqual([...config.allowedGroupIds], ['123', '456'])
  assert.equal(config.commands.has('状态检查'), true)
})
