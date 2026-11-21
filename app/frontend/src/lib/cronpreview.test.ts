import { describe, expect, it } from 'vitest'
import { cronPreviewLine } from './cronpreview'

describe('cronPreviewLine', () => {
  it('says the words and the next firings', () => {
    // Before: a mistake in the cron line showed only after Save.
    const p = { words: 'On weekdays at 09:00', next: ['Tue 09:00', 'Wed 09:00'], error: '' }
    expect(cronPreviewLine(p)).toEqual({ text: 'On weekdays at 09:00. Next: Tue 09:00, Wed 09:00.', error: false })
  })
  it('lists the firings alone when the line is not a common pattern', () => {
    expect(cronPreviewLine({ words: '', next: ['Tue 09:07'], error: '' })).toEqual({ text: 'Next: Tue 09:07', error: false })
  })
  it('shows the reason, as an error, when the line cannot be used', () => {
    expect(cronPreviewLine({ words: '', next: [], error: 'cron minute: 61 is outside 0-59' })).toEqual({ text: 'cron minute: 61 is outside 0-59', error: true })
  })
  it('has nothing to say for an empty line', () => {
    expect(cronPreviewLine({ words: '', next: [], error: '' }).text).toBe('')
  })
})
