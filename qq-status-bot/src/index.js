import 'dotenv/config'
import WebSocket from 'ws'
import { loadConfig } from './config.js'
import { fetchMonitorStatus } from './site.js'
import { renderStatus } from './render.js'
import { captureStatusPage } from './screenshot.js'

const config = loadConfig()
const cooldowns = new Map()
const pending = new Map()
let sequence = 0

function oneBotUrl() {
  const url = new URL(config.onebotWsUrl)
  url.searchParams.set('access_token', config.onebotToken)
  return url.toString()
}

function call(ws, action, params) {
  const echo = `bot-${++sequence}`
  ws.send(JSON.stringify({ action, params, echo }))
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(echo)
      reject(new Error(`OneBot action timed out: ${action}`))
    }, 20_000)
    pending.set(echo, { resolve, reject, timer })
  })
}

async function handleGroupMessage(ws, event) {
  const groupId = String(event.group_id || '')
  const text = String(event.raw_message || '').trim()
  if (!config.allowedGroupIds.has(groupId) || !config.commands.has(text)) return

  const lastRun = cooldowns.get(groupId) || 0
  if (Date.now() - lastRun < config.cooldownMs) return
  cooldowns.set(groupId, Date.now())

  await call(ws, 'send_group_msg', { group_id: event.group_id, message: '正在获取站点状态…' })
  try {
    let image
    try {
      image = await captureStatusPage(config)
    } catch (screenshotError) {
      console.error('Real page screenshot failed; using rendered fallback:', screenshotError)
      const items = await fetchMonitorStatus(config)
      image = await renderStatus(items)
    }
    await call(ws, 'send_group_msg', {
      group_id: event.group_id,
      message: [{ type: 'image', data: { file: `base64://${image.toString('base64')}` } }],
    })
  } catch (error) {
    console.error('Status check failed:', error)
    try {
      await call(ws, 'send_group_msg', {
        group_id: event.group_id,
        message: `状态检查失败：${error.message}`,
      })
    } catch (notifyError) {
      console.error('Failed to notify group about status error:', notifyError)
    }
  }
}

function connect() {
  const ws = new WebSocket(oneBotUrl())
  ws.on('open', () => console.log('Connected to NapCat OneBot'))
  ws.on('message', data => {
    let event
    try { event = JSON.parse(data.toString()) } catch { return }
    if (event.echo && pending.has(event.echo)) {
      const task = pending.get(event.echo)
      clearTimeout(task.timer)
      pending.delete(event.echo)
      event.status === 'ok' ? task.resolve(event.data) : task.reject(new Error(event.message || event.wording || 'OneBot action failed'))
      return
    }
    if (event.post_type === 'message' && event.message_type === 'group') {
      void handleGroupMessage(ws, event)
    }
  })
  ws.on('close', () => {
    console.error('NapCat connection closed; reconnecting in 5 seconds')
    setTimeout(connect, 5000)
  })
  ws.on('error', error => console.error('NapCat WebSocket error:', error.message))
}

connect()
