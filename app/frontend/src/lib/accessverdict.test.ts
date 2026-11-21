import { describe, expect, it } from 'vitest'
import { verdictText } from './accessverdict'

describe('verdictText', () => {
  it('says a refusal is as it should be', () => {
    expect(verdictText('denied', 403)).toMatchObject({ mark: '✓', text: '403 refused, as it should be' })
  })
  it('says being let in is not protected', () => {
    expect(verdictText('allowed', 200)).toMatchObject({ mark: '✗', text: '200 let in: not protected' })
  })
  it('says an unclear answer is unclear, and leaves out a missing status', () => {
    expect(verdictText('unclear', 0)).toMatchObject({ mark: '?', text: 'unclear' })
  })
})
