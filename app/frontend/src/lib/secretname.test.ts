import { describe, expect, it } from 'vitest'
import { secretLike } from './secretname'

describe('secretLike', () => {
  it('flags names that say secret', () => {
    for (const n of ['token', 'API_TOKEN', 'password', 'db_pwd', 'api_key', 'apiKey', 'accessToken', 'private_key', 'client_secret', 'x-api-key'])
      expect(secretLike(n, ''), n).not.toBe('')
  })

  it('does not flag names that only describe one', () => {
    for (const n of ['token_url', 'password_policy_name', 'secret_id', 'base_url', 'key', 'tokens_per_page_ttl', 'user'])
      expect(secretLike(n, ''), n).toBe('')
  })

  it('flags values with a well-known secret shape', () => {
    // Before: only the name was ever looked at, so base_header=sk_live_… was saved in plain text.
    expect(secretLike('x', 'sk_live_abc123')).toMatch(/value/)
    expect(secretLike('x', 'eyJhbGciOi.eyJzdWIi.sig')).toMatch(/value/)
    expect(secretLike('x', 'https://api.example.com')).toBe('')
  })
})
