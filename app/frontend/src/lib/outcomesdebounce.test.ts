import { describe, expect, it } from 'vitest'
import editor from '../OutcomesEditor.svelte?raw'
import checks from '../ChecksScreen.svelte?raw'

describe('OutcomesEditor typing', () => {
  it('Given typing in a name, rule or description Then the write waits for a pause and is flushed on leaving', () => {
    // Before: every keystroke was an edit of the flow, so one undo went back one letter.
    expect(editor).toContain('const commitSoon = debounced(send, 400)')
    expect(editor).toContain('oninput={() => commitSoon()}')
    expect(editor).toContain('commitSoon.flush()')
    expect(editor).not.toContain('oninput={() => commit()}')
  })
  it('Given a switch, move or removal Then it writes at once and takes the pending text with it', () => {
    expect(editor).toContain('commitSoon.cancel()')
  })
})

describe('Checks screen captions', () => {
  it('Given the Checks screen Then the long explanations sit behind a ? and the captions are one line', () => {
    // Before: three paragraphs stood above any result.
    expect(checks.match(/<Hint>/g)?.length).toBeGreaterThanOrEqual(3)
    expect(checks).not.toContain('<p class="note">Checks look at the OpenAPI descriptions you imported. Use Coverage')
  })
})
