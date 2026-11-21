import { describe, expect, it } from 'vitest'
import inspector from '../Inspector.svelte?raw'
import footer from '../Footer.svelte?raw'
import fold from '../FoldSection.svelte?raw'

describe('Inspector sections', () => {
  it('Given a step When inspected Then Run when, Timeout and failure start open in their own section', () => {
    // Before: they sat in a "General" section that was shut.
    expect(inspector).toContain('<FoldSection name="step-when" title="When & failure" open>')
    expect(inspector).not.toContain('<summary>General</summary>')
    const when = inspector.slice(inspector.indexOf('name="step-when"'), inspector.indexOf('name="step-identity"'))
    for (const field of ['Run when (expression)', 'Timeout (e.g. 5s)', 'If this step fails']) expect(when).toContain(field)
  })
  it('Given a section When toggled Then the choice is kept by name', () => {
    expect(fold).toContain('readFold(store(), name, open)')
    expect(fold).toContain('writeFold(store(), name, now)')
  })
})

describe('Footer panel', () => {
  it('Given an open panel Then it has a handle, a kept height and shortened step names', () => {
    // Before: the panel was 220 px tall for good and a long step name wrapped over the message.
    expect(footer).toContain('<Splitter name="footer-height" axis="y"')
    expect(footer).toContain('style:height={`${height}px`}')
    expect(footer).not.toContain('height: 220px')
    expect(footer).toContain('text-overflow: ellipsis')
  })
  it('Given work in progress Then the footer says what is running', () => {
    expect(footer).toContain('runChip(Object.values(app.flows), Object.values(app.requests))')
  })
})
