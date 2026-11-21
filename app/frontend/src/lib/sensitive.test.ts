import { describe, expect, it } from 'vitest'
import { sensitiveName, sensitiveValue } from './sensitive'

// Before: nothing hid a credential on screen, so a talk or a screenshot showed the Authorization header whole.
describe('sensitive values', () => {
  it('Given credential headers in any case Then they are sensitive', () => {
    for (const n of ['Authorization', 'cookie', 'Set-Cookie', 'X-API-Key', ' Proxy-Authorization ']) expect(sensitiveName(n), n).toBe(true)
  })
  it('Given names that say token or password Then they are sensitive, and plain ones are not', () => {
    expect(sensitiveName('access_token')).toBe(true)
    expect(sensitiveName('Content-Type')).toBe(false)
    expect(sensitiveName('base_url')).toBe(false)
    expect(sensitiveName('token_url')).toBe(false)
  })
  it('Given a value shaped like a credential Then it is sensitive whatever its name', () => {
    expect(sensitiveValue('x', 'ghp_abcdefghijklmnopqrstuvwxyz0123456789')).toBe(true)
    expect(sensitiveValue('x', 'eyJhbGciOi.eyJzdWIiOi.sig')).toBe(true)
    expect(sensitiveValue('Accept', 'application/json')).toBe(false)
  })
})
