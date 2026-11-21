import { describe, expect, it } from 'vitest'
import { LOADING, READY, failed, mayShowEmpty } from './loadstate'
import secrets from '../SecretsScreen.svelte?raw'

describe('load state', () => {
  it('shows the empty state only for data that came and is empty', () => {
    // Before: a failed or unfinished load showed "No secret is declared yet".
    expect(mayShowEmpty(READY, 0)).toBe(true)
    expect(mayShowEmpty(READY, 2)).toBe(false)
    expect(mayShowEmpty(LOADING, 0)).toBe(false)
    expect(mayShowEmpty(failed(new Error('cancelled')), 0)).toBe(false)
  })

  it('keeps the message of a failure', () => {
    const s = failed(new Error('the passphrase was cancelled'))

    expect(s.status).toBe('error')
    expect(s.status === 'error' && s.error).toContain('passphrase')
  })

  it('is what the secrets screen reads: an error with Try again, and no false empty state', () => {
    expect(secrets).toContain('Try again')
    expect(secrets).toContain('mayShowEmpty(load, view.secrets.length)')
    expect(secrets).toContain('(e) => (load = failed(e))')
  })
})
