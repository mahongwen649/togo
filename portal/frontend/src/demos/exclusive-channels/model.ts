export type Phase = 'available' | 'active' | 'releasing' | 'cooling' | 'maintenance' | 'unpublished'
export type ReleaseReason = 'manual' | 'expired' | 'admin' | 'idle'
export interface DemoUser { id: number; name: string; balance: number }
export interface Account { id?: number; name: string; available: boolean; fiveHour: number; weekly: number }
export const accountCandidates: (Account & { id: number })[] = [
  { id: 1001, name: 'OpenAI 账号 01', available: true, fiveHour: 12, weekly: 28 },
  { id: 1002, name: 'OpenAI 账号 02', available: true, fiveHour: 24, weekly: 35 },
  { id: 1003, name: 'OpenAI 账号 03', available: true, fiveHour: 8, weekly: 19 },
  { id: 1004, name: 'OpenAI 账号 04', available: true, fiveHour: 36, weekly: 52 },
  { id: 1005, name: 'OpenAI 账号 05', available: false, fiveHour: 100, weekly: 68 },
]
export function selectableAccounts(state: DemoState) {
  const boundIds = new Set(state.groups.flatMap(g => g.accounts.map(a => a.id)))
  return accountCandidates.filter(a => a.available && !boundIds.has(a.id))
}
export interface Group {
  id: number; name: string; platform: string; models: string[]; multiplier: number
  accounts: Account[]; eligible: boolean; issue?: string
}
export interface Request { id: number; endsAt: number; cost: number; tokens: number }
export interface Lease {
  id: number; userId: number; startedAt: number; expiresAt: number; duration: number
  key: string; requests: Request[]; cost: number; tokens: number; completed: number
  cleanupAt?: number; reason?: ReleaseReason
  lastChargedAt?: number
}
export interface Channel {
  id: number; groupId: number; name: string; durations: number[]; cooldownSeconds: number
  phase: Phase; lease: Lease | null; cooldownUntil: number | null
  todayCost: number; todayRequests: number; todayTokens: number; updatedAt: number
}
export interface History {
  id: number; channelName: string; userId: number; startedAt: number; endedAt: number
  reason: ReleaseReason; cost: number; tokens: number; requests: number
}
export interface Event { id: number; at: number; channelName: string; action: string; actor: string }
export interface DemoState {
  version: 1; now: number; realSavedAt: number; nextId: number; users: DemoUser[]
  groups: Group[]; channels: Channel[]; history: History[]; events: Event[]
}
export const STORAGE_KEY = 'portal:exclusive-channels-demo:v1'
export const DEFAULT_DURATION = 24 * 60
export const IDLE_TIMEOUT = 30 * 60000
export function idleDeadline(lease: Lease): number {
  return (lease.lastChargedAt ?? lease.startedAt) + IDLE_TIMEOUT
}
export const durationOptions = [30, 60, 120, 240, 480, 720, 1440]
export const phaseNames: Record<Phase, string> = {
  available: '空闲', active: '使用中', releasing: '归还中', cooling: '冷却中',
  maintenance: '维护中', unpublished: '已下架',
}
export const reasonNames: Record<ReleaseReason, string> = { manual: '主动归还', expired: '到期回收', admin: '管理员回收', idle: '30 分钟无消费回收' }
export function durationLabel(minutes: number): string {
  return minutes < 60 ? `${minutes} 分钟` : `${minutes / 60} 小时`
}
export function countdown(until: number, now: number): string {
  const seconds = Math.max(0, Math.ceil((until - now) / 1000))
  return [Math.floor(seconds / 3600), Math.floor(seconds % 3600 / 60), seconds % 60]
    .map(n => String(n).padStart(2, '0')).join(':')
}
export function createDemo(now = Date.now()): DemoState {
  const accounts = (count: number, unavailable = 0): Account[] => Array.from({ length: count }, (_, i) => ({
    name: `账号 ${String.fromCharCode(65 + i)}`, available: i >= unavailable,
    fiveHour: i < unavailable ? 100 : [28, 14, 42, 8][i % 4]!, weekly: [46, 31, 62, 20][i % 4]!,
  }))
  const groups: Group[] = [
    { id: 101, name: 'OpenAI 开发组', platform: 'OpenAI', models: ['GPT-5', 'GPT-5 mini'], multiplier: 1, accounts: accounts(3), eligible: true },
    { id: 102, name: 'OpenAI 长任务组', platform: 'OpenAI', models: ['GPT-5', 'GPT-5 mini'], multiplier: 1.2, accounts: accounts(4, 1), eligible: true },
    { id: 103, name: 'OpenAI 日常组', platform: 'OpenAI', models: ['GPT-5 mini'], multiplier: 0.8, accounts: accounts(2), eligible: true },
    { id: 104, name: 'OpenAI 弹性组', platform: 'OpenAI', models: ['GPT-5', 'GPT-5 mini'], multiplier: 1, accounts: accounts(3), eligible: true },
    { id: 105, name: 'OpenAI 备用组', platform: 'OpenAI', models: ['GPT-5 mini'], multiplier: 0.8, accounts: accounts(2, 2), eligible: true },
    { id: 106, name: 'OpenAI 新增组', platform: 'OpenAI', models: ['GPT-5', 'GPT-5 mini'], multiplier: 1, accounts: accounts(2), eligible: true },
    { id: 107, name: '普通中转共享组', platform: 'OpenAI', models: ['GPT-5'], multiplier: 1, accounts: accounts(2), eligible: false, issue: '非专属分组，账号与普通中转共享' },
  ]
  const names = ['开发通道 01', '长任务通道 02', '轻量通道 03', '弹性通道 04', '备用通道 05']
  const state: DemoState = {
    version: 1, now, realSavedAt: Date.now(), nextId: 20,
    users: [{ id: 1, name: '林一', balance: 128.5 }, { id: 2, name: '陈舟', balance: 86.2 }, { id: 3, name: '余额不足用户', balance: 0 }],
    groups, channels: names.map((name, i) => ({
      id: i + 1, groupId: 101 + i, name, durations: [30, 60, 120, 240, 480, 1440], cooldownSeconds: i === 1 ? 120 : 60,
      phase: i === 4 ? 'maintenance' : 'available', lease: null, cooldownUntil: null,
      todayCost: [18.62, 32.8, 6.54, 14.2, 0][i]!, todayRequests: [428, 712, 184, 316, 0][i]!,
      todayTokens: [1246000, 2832000, 482000, 916000, 0][i]!, updatedAt: now - 12000,
    })), history: [], events: [],
  }
  state.channels[1]!.phase = 'active'
  state.channels[1]!.lease = {
    id: 10, userId: 2, startedAt: now - 2 * 3600000, expiresAt: now + 2 * 3600000, duration: 240, lastChargedAt: now - 10 * 60000,
    key: 'demo-only-channel-02-lease-10', requests: [], cost: 4.32, tokens: 148000, completed: 38,
  }
  state.channels[3]!.phase = 'cooling'
  state.channels[3]!.cooldownUntil = now + 60000
  state.history = [{ id: 9, channelName: names[2]!, userId: 1, startedAt: now - 5 * 3600000,
    endedAt: now - 4 * 3600000, reason: 'manual', cost: 2.16, tokens: 72000, requests: 18 }]
  record(state, state.channels[1]!, '领取通道 · 4 小时', '陈舟', now - 2 * 3600000)
  record(state, state.channels[3]!, '开始冷却 · 60 秒', '系统')
  return state
}
export function groupOf(state: DemoState, channel: Channel): Group {
  const group = state.groups.find(g => g.id === channel.groupId)
  if (!group) throw new Error('分组不存在')
  return group
}
export function occupiedBy(state: DemoState, userId: number): Channel | undefined {
  return state.channels.find(c => c.lease?.userId === userId)
}
function record(state: DemoState, channel: Channel, action: string, actor: string, at = state.now) {
  state.events.unshift({ id: state.nextId++, at, channelName: channel.name, action, actor })
  state.events = state.events.slice(0, 150)
}
export function claim(state: DemoState, channelId: number, userId: number, duration = DEFAULT_DURATION): Channel {
  const user = state.users.find(u => u.id === userId)
  const channel = state.channels.find(c => c.id === channelId)
  if (!user || user.balance <= 0) throw new Error('中转余额不足，无法领取通道')
  if (occupiedBy(state, userId)) throw new Error('你已有通道，请先归还当前通道')
  if (!channel || channel.phase !== 'available') throw new Error('此通道当前不可领取')
  if (!channel.durations.includes(duration)) throw new Error('请选择管理员允许的使用时长')
  const group = groupOf(state, channel)
  if (!group.eligible || !group.accounts.some(a => a.available)) throw new Error('此通道暂无可用账号')
  const id = state.nextId++
  channel.lease = { id, userId, startedAt: state.now, expiresAt: state.now + duration * 60000, duration,
    key: `demo-only-channel-${channel.id}-lease-${id}`, requests: [], cost: 0, tokens: 0, completed: 0 }
  channel.phase = 'active'
  record(state, channel, `领取通道 · ${durationLabel(duration)}`, user.name)
  return channel
}
export function release(state: DemoState, channelId: number, userId: number, reason: ReleaseReason = 'manual') {
  const channel = state.channels.find(c => c.id === channelId)
  if (!channel?.lease || channel.phase !== 'active') throw new Error('此通道没有可归还的使用记录')
  if (reason === 'manual' && channel.lease.userId !== userId) throw new Error('不能归还其他用户的通道')
  channel.phase = 'releasing'
  channel.lease.cleanupAt = state.now + 3000
  channel.lease.reason = reason
  record(state, channel, `${reasonNames[reason]} · 停止接收新请求`, reason === 'manual' ? state.users.find(u => u.id === userId)!.name : reason === 'admin' ? '管理员' : '系统')
}
export function simulateRequest(state: DemoState, userId: number) {
  const channel = occupiedBy(state, userId)
  if (!channel?.lease || channel.phase !== 'active' || Math.min(channel.lease.expiresAt, idleDeadline(channel.lease)) <= state.now) throw new Error('当前通道不可发起请求')
  const user = state.users.find(u => u.id === userId)!
  if (user.balance <= 0) throw new Error('中转余额不足')
  if (channel.lease.requests.length >= 3) throw new Error('演示并发已达上限')
  const cost = Math.round(0.12 * groupOf(state, channel).multiplier * 100) / 100
  channel.lease.requests.push({ id: state.nextId++, endsAt: state.now + 20000, cost, tokens: 4800 })
  record(state, channel, '模拟请求开始 · 20 秒', user.name)
}
// Deadlines are processed chronologically: time jumps cannot bypass draining
// or start cooldown before the final in-flight request finishes.
export function advance(state: DemoState, target: number) {
  if (!Number.isFinite(target) || target < state.now) return
  for (;;) {
    const deadlines = state.channels.flatMap(c => [
      ...(c.phase === 'active' && c.lease ? [Math.min(c.lease.expiresAt, idleDeadline(c.lease))] : []),
      ...(c.lease?.requests.map(r => r.endsAt) ?? []),
      ...(c.phase === 'releasing' && c.lease?.cleanupAt ? [Math.max(c.lease.cleanupAt, ...c.lease.requests.map(r => r.endsAt))] : []),
      ...(c.phase === 'cooling' && c.cooldownUntil !== null ? [c.cooldownUntil] : []),
    ]).filter(t => t <= target)
    if (!deadlines.length) break
    state.now = Math.max(state.now, Math.min(...deadlines))
    for (const channel of state.channels) {
      const lease = channel.lease
      if (lease) {
        for (const request of lease.requests.filter(r => r.endsAt <= state.now)) {
          lease.cost = Math.round((lease.cost + request.cost) * 100) / 100
          lease.tokens += request.tokens
          lease.completed++
          if (request.cost > 0) lease.lastChargedAt = state.now
          const user = state.users.find(u => u.id === lease.userId)!
          user.balance = Math.round((user.balance - request.cost) * 100) / 100
          channel.todayCost = Math.round((channel.todayCost + request.cost) * 100) / 100
          channel.todayTokens += request.tokens
          channel.todayRequests++
          channel.updatedAt = state.now
          record(state, channel, `模拟请求完成 · ¥${request.cost.toFixed(2)}`, user.name)
        }
        lease.requests = lease.requests.filter(r => r.endsAt > state.now)
        if (channel.phase === 'active' && lease.expiresAt <= state.now) release(state, channel.id, lease.userId, 'expired')
        if (channel.phase === 'active' && idleDeadline(lease) <= state.now) release(state, channel.id, lease.userId, 'idle')
        if (channel.phase === 'releasing' && lease.cleanupAt! <= state.now && !lease.requests.length) {
          state.history.unshift({ id: lease.id, channelName: channel.name, userId: lease.userId, startedAt: lease.startedAt,
            endedAt: state.now, reason: lease.reason!, cost: lease.cost, tokens: lease.tokens, requests: lease.completed })
          channel.phase = 'cooling'
          channel.cooldownUntil = state.now + channel.cooldownSeconds * 1000
          channel.lease = null
          record(state, channel, `清理完成 · 冷却 ${channel.cooldownSeconds} 秒`, '系统')
        }
      }
      if (channel.phase === 'cooling' && channel.cooldownUntil! <= state.now) {
        channel.phase = 'available'
        channel.cooldownUntil = null
        record(state, channel, '冷却结束 · 可领取', '系统')
      }
    }
  }
  state.now = target
}
export interface ChannelConfig { id?: number; groupId?: number; accountIds?: number[]; name: string; durations: number[]; cooldownSeconds: number }
export function saveChannel(state: DemoState, config: ChannelConfig) {
  const name = config.name.trim()
  if (!name || name.length > 40) throw new Error('通道名称必须为 1 到 40 个字符')
  if (!Number.isInteger(config.cooldownSeconds) || config.cooldownSeconds < 0 || config.cooldownSeconds > 86400) throw new Error('冷却时间须为 0 到 86400 秒的整数')
  if (!config.durations.includes(DEFAULT_DURATION) || config.durations.some(d => !durationOptions.includes(d))) throw new Error('可选时长须包含默认的 24 小时')
  let channel = state.channels.find(c => c.id === config.id)
  if (config.id && !channel) throw new Error('通道不存在')
  if (channel && config.groupId !== undefined && channel.groupId !== config.groupId) throw new Error('已有通道不能更换分组')
  if (channel && config.accountIds !== undefined) throw new Error('已有通道暂不支持更换绑定账号')
  if (!channel) {
    if (config.groupId !== undefined) throw new Error('发布时请选择账号，分组将自动创建')
    const ids = config.accountIds ?? []
    if (!ids.length) throw new Error('请至少选择一个可用账号')
    if (new Set(ids).size !== ids.length) throw new Error('不能重复选择账号')
    const available = selectableAccounts(state)
    const selected = ids.map(id => available.find(a => a.id === id))
    if (selected.some(a => !a)) throw new Error('所选账号已绑定或不可用，请重新选择')
    const groupId = Math.max(0, ...state.groups.map(g => g.id)) + 1
    const group: Group = { id: groupId, name: `独享 · ${name}`, platform: 'OpenAI',
      models: ['GPT-5', 'GPT-5 mini'], multiplier: 1, eligible: true,
      accounts: selected.map(a => ({ ...a! })) }
    channel = { id: state.nextId++, groupId, name, durations: [], cooldownSeconds: 0,
      phase: 'available', lease: null, cooldownUntil: null, todayCost: 0, todayRequests: 0, todayTokens: 0, updatedAt: state.now }
    state.groups.push(group)
    state.channels.push(channel)
    record(state, channel, `自动创建 Core 分组 #${groupId} · 绑定 ${ids.length} 个账号`, '管理员')
  }
  Object.assign(channel, { name, durations: [...new Set(config.durations)].sort((a, b) => a - b), cooldownSeconds: config.cooldownSeconds })
  record(state, channel, config.id ? '更新通道配置' : '发布通道', '管理员')
}
export function setAvailability(state: DemoState, channelId: number, phase: 'available' | 'maintenance' | 'unpublished') {
  const channel = state.channels.find(c => c.id === channelId)
  if (!channel || channel.lease || channel.phase === 'cooling') throw new Error('占用或冷却期间不能更改发布状态')
  if (phase === 'available' && !groupOf(state, channel).accounts.some(a => a.available)) throw new Error('分组暂无可用账号，不能开放领取')
  channel.phase = phase
  record(state, channel, `设置为${phaseNames[phase]}`, '管理员')
}
export function nextTransition(state: DemoState, userId: number): number {
  const mine = occupiedBy(state, userId)
  if (mine?.phase === 'active') return Math.min(mine.lease!.expiresAt, idleDeadline(mine.lease!))
  const times = state.channels.flatMap(c => [
    ...(c.phase === 'active' && c.lease ? [Math.min(c.lease.expiresAt, idleDeadline(c.lease))] : []),
    ...(c.phase === 'releasing' && c.lease ? [Math.max(c.lease.cleanupAt!, ...c.lease.requests.map(r => r.endsAt))] : []),
    ...(c.cooldownUntil !== null ? [c.cooldownUntil] : []),
  ])
  return times.length ? Math.min(...times) : state.now + 60000
}
