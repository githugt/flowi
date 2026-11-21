import { describe, expect, it } from 'vitest'
import feed from '../MessageFeed.svelte?raw'
import detail from '../MessageDetail.svelte?raw'

describe('message detail wiring', () => {
  it('should open a row in the detail viewer, not a flat text block', () => {
    // Before: the opened row was a Copy button, a headers table and Code, with no tree, hex or partition and offset.
    expect(feed).toContain('<MessageDetail message={m} />')
    expect(feed).not.toContain('<Code')
  })

  it('should measure a row from the message, so a binary one shows its real size', () => {
    expect(feed).toContain('bodyBytes(m)')
  })

  it('should draw the tree with the JSON tree component and switch modes with buttons', () => {
    expect(detail).toContain('<JsonNode value={tree.value}')
    expect(detail).toMatch(/<Button small pressed=\{mode === m\}/)
    expect(detail).not.toMatch(/<button/)
  })

  it('should offer Copy as… with the forms the message allows', () => {
    expect(detail).toContain('label="Copy as…"')
    expect(detail).toContain('options={copyChoices(message)}')
  })
})
