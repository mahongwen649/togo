import { describe, expect, it } from 'vitest'
import { defaultCheckSchedule, parseCheckSchedule, scheduleCron } from '../checkSchedule'

describe('account check schedule', () => {
  it.each([
    [{ frequency: 'minutes', interval: 15 }, 'CRON_TZ=Asia/Shanghai */15 * * * *'],
    [{ frequency: 'hourly', minute: 45 }, 'CRON_TZ=Asia/Shanghai 45 * * * *'],
    [{ frequency: 'daily', time: '00:05' }, 'CRON_TZ=Asia/Shanghai 5 0 * * *'],
    [{ frequency: 'weekly', time: '23:59', weekday: 0 }, 'CRON_TZ=Asia/Shanghai 59 23 * * 0']
  ] as const)('round-trips a visible schedule without losing time or weekday: %s', (fields, cron) => {
    const schedule = { ...defaultCheckSchedule(), ...fields }
    expect(scheduleCron(schedule)).toBe(cron)
    expect(parseCheckSchedule(cron)).toMatchObject(fields)
  })

  it.each(['0 9 * * *', 'CRON_TZ=UTC 0 9 * * 1-5', 'CRON_TZ=Asia/Shanghai 0 9 1 * *'])('preserves existing advanced expression %s', cron => {
    const parsed = parseCheckSchedule(cron)
    expect(parsed.frequency).toBe('custom')
    expect(scheduleCron(parsed)).toBe(cron)
  })

  it('rejects incomplete or out-of-range time inputs', () => {
    for (const fields of [{ frequency: 'hourly' as const, minute: 60 }, { frequency: 'hourly' as const, minute: 1.5 }, { frequency: 'daily' as const, time: '' }, { frequency: 'daily' as const, time: '24:00' }]) {
      expect(scheduleCron({ ...defaultCheckSchedule(), ...fields })).toBe('')
    }
  })
})
