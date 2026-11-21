import { describe, expect, it } from 'vitest'
import { usedByText, visibleSecrets } from './secretlist'

const s = (name: string, state: 'missing' | 'stored' | 'overridden') => ({ environment: 'staging', name, state, usedBy: [] })

describe('the secrets list', () => {
  it('should narrow to the secrets with no value', () => {
    // Before: finding the secret a run would stop at meant reading every row of every environment.
    const all = [s('a', 'stored'), s('b', 'missing'), s('c', 'overridden')]

    expect(visibleSecrets(all, true).map((x) => x.name)).toEqual(['b'])
    expect(visibleSecrets(all, false)).toHaveLength(3)
  })

  it('should say what uses a secret, by kind', () => {
    const use = (kind: string) => ({ kind, label: 'x', node: '' })

    expect(usedByText([use('request'), use('request'), use('flow'), use('auth')])).toBe('Used by 4: 2 requests, 1 flow, 1 authentication')
    expect(usedByText([use('flow')])).toBe('Used by 1: 1 flow')
    expect(usedByText([])).toBe('Not used by any file')
    expect(usedByText(undefined)).toBe('Not used by any file')
  })
})
