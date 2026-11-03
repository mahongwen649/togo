export type ScheduleFrequency = 'minutes' | 'hourly' | 'daily' | 'weekly' | 'custom'

export interface CheckSchedule {
  frequency: ScheduleFrequency
  interval: number
  minute: number
  time: string
  weekday: number
  custom: string
}

export const weekdays = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
export const scheduleTimezone = 'CRON_TZ=Asia/Shanghai '

export function defaultCheckSchedule(): CheckSchedule {
  return { frequency: 'hourly', interval: 30, minute: 0, time: '09:00', weekday: 1, custom: '' }
}

export function scheduleCron(schedule: CheckSchedule): string {
  const { frequency, interval, minute, time, weekday } = schedule
  if (frequency === 'custom') return schedule.custom.trim()
  if (frequency === 'minutes') {
    return [5, 10, 15, 20, 30].includes(interval) ? `${scheduleTimezone}*/${interval} * * * *` : ''
  }
  if (frequency === 'hourly') {
    return Number.isInteger(minute) && minute >= 0 && minute <= 59 ? `${scheduleTimezone}${minute} * * * *` : ''
  }
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(time)) return ''
  const [hour, minutes] = time.split(':').map(Number)
  if (frequency === 'daily') return `${scheduleTimezone}${minutes} ${hour} * * *`
  return Number.isInteger(weekday) && weekday >= 0 && weekday <= 6 ? `${scheduleTimezone}${minutes} ${hour} * * ${weekday}` : ''
}

export function parseCheckSchedule(expression: string): CheckSchedule {
  const schedule = defaultCheckSchedule()
  const custom = { ...schedule, frequency: 'custom' as const, custom: expression }
  // Keep legacy/server-time and advanced expressions intact when editing.
  if (!expression.startsWith(scheduleTimezone)) return custom
  const parts = expression.slice(scheduleTimezone.length).trim().split(/\s+/)
  if (parts.length !== 5) return custom
  const [minute, hour, day, month, weekday] = parts
  if (day !== '*' || month !== '*') return custom
  if (/^\*\/(5|10|15|20|30)$/.test(minute) && hour === '*' && weekday === '*') {
    return { ...schedule, frequency: 'minutes', interval: Number(minute.slice(2)) }
  }
  if (!/^\d+$/.test(minute) || Number(minute) > 59) return custom
  if (hour === '*' && weekday === '*') return { ...schedule, minute: Number(minute) }
  if (!/^\d+$/.test(hour) || Number(hour) > 23) return custom
  const time = `${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`
  if (weekday === '*') return { ...schedule, frequency: 'daily', time }
  if (/^[0-6]$/.test(weekday)) return { ...schedule, frequency: 'weekly', time, weekday: Number(weekday) }
  return custom
}

export function scheduleDescription(schedule: CheckSchedule): string {
  switch (schedule.frequency) {
    case 'minutes': return `每隔 ${schedule.interval} 分钟（按整点对齐）`
    case 'hourly': return `每小时的第 ${schedule.minute} 分钟`
    case 'daily': return `每天 ${schedule.time}`
    case 'weekly': return `每${weekdays[schedule.weekday]} ${schedule.time}`
    case 'custom': return `自定义时间：${schedule.custom}`
  }
}
