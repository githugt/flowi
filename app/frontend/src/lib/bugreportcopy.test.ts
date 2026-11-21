import { describe, expect, it } from 'vitest'
import { bugReportNote } from './bugreportcopy'
import pane from '../ResponsePane.svelte?raw'

describe('bugReportNote', () => {
  it('names what was masked, a few at a time', () => {
    expect(bugReportNote(['Authorization header (request)'])).toBe('Copied for a bug report. Masked: Authorization header (request). Read it before you share it.')
    expect(bugReportNote(['a', 'b', 'c', 'd', 'e', 'f'])).toContain('Masked: a, b, c, d, and 2 more.')
  })

  it('says so when nothing looked like a credential', () => {
    expect(bugReportNote([])).toContain('Nothing looked like a credential')
  })
})

describe('the response pane', () => {
  it('has the button, and shows the masking in its message', () => {
    expect(pane).toContain('Copy for a bug report')
    expect(pane).toContain('RequestService.BugReport(view.node)')
    expect(pane).toContain('bugReportNote(report.masked)')
    expect(pane).not.toContain('ui.toast(bugReportNote') // one toast, not "Copied" and then the note
  })
})
