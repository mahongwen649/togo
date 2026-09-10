import { describe, expect, it } from 'vitest'
import { extractI18nErrorMessage } from '@/utils/apiError'

describe('extractI18nErrorMessage', () => {
  it('localizes the legacy English email verification message', () => {
    const t = (key: string) => key === 'auth.errors.EMAIL_VERIFY_REQUIRED'
      ? '请先完成邮箱验证'
      : key

    expect(
      extractI18nErrorMessage(
        { code: 400, message: 'email verification is required' },
        t,
        'auth.errors',
        '注册失败，请重试'
      )
    ).toBe('请先完成邮箱验证')
  })

  it('uses a reason code before falling back to the raw message', () => {
    const t = (key: string) => key === 'auth.errors.EMAIL_VERIFY_REQUIRED'
      ? '请先完成邮箱验证'
      : key

    expect(
      extractI18nErrorMessage(
        { reason: 'EMAIL_VERIFY_REQUIRED', message: 'email verification is required' },
        t,
        'auth.errors',
        '注册失败，请重试'
      )
    ).toBe('请先完成邮箱验证')
  })
})
