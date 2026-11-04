import { describe, expect, it } from 'vitest'
import { problemsOf, resultTone, storyText } from './story'
import type { ExplainView } from './types'

const sample: ExplainView = {
  sentences: [
    { indent: 0, text: 'Call auth/login', node: 'login' },
    { indent: 0, text: 'If ok:', node: 'cond' },
    { indent: 1, text: 'Call a', node: 'a' },
  ],
  problems: [{ node: 'login', text: 'token is set but nothing reads it' }],
}

describe('story', () => {
  it('writes the sentences indented two spaces a level, then the problems', () => {
    expect(storyText(sample)).toBe(['Call auth/login', 'If ok:', '  Call a', '', 'Problems:', '  - token is set but nothing reads it (step login)'].join('\n'))
  })

  it('writes just the sentences when there are no problems, and nothing for an empty flow', () => {
    expect(storyText({ sentences: [{ indent: 0, text: 'Wait 2s', node: 'w' }], problems: [] })).toBe('Wait 2s')
    expect(storyText({ sentences: [], problems: [] })).toBe('')
  })

  it('takes a tone from how the step ended', () => {
    expect(resultTone('passed · 120ms')).toBe('pass')
    expect(resultTone('failed · 500')).toBe('fail')
    expect(resultTone(undefined)).toBe('')
    expect(resultTone('skipped')).toBe('')
  })

  it('finds the problems of one step', () => {
    expect(problemsOf(sample, 'login')).toEqual(['token is set but nothing reads it'])
    expect(problemsOf(sample, 'a')).toEqual([])
  })
})
