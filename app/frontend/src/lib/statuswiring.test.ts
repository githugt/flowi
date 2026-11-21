import { describe, expect, it } from 'vitest'
import status from '../Status.svelte?raw'
import css from '../app.css?raw'

const users = import.meta.glob('../{TrafficLog,ResponsePane,ReplayerScreen,SecretsScreen}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Status', () => {
  it('is a dot with the words, so a status never rests on colour alone, in four tones and as a pill or plain', () => {
    expect(status).toContain("tone?: '' | 'ok' | 'warn' | 'bad' | 'info'")
    expect(status).toContain('.status::before')
    expect(status).toContain('plain')
  })

  it('is what the response code, the call code, the replayer state and a secret state use', () => {
    // Before: a global `.status-pill` class in app.css, and three screens that drew their own dot or coloured word.
    expect(Object.keys(users)).toHaveLength(4)
    for (const [path, text] of Object.entries(users)) expect(text, path).toContain('<Status')
    expect(css).not.toContain('.status-pill')
    for (const text of Object.values(users)) expect(text).not.toContain('status-pill')
  })
})
