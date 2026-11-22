import { describe, expect, it } from 'vitest'
import inspector from '../Inspector.svelte?raw'
import footer from '../Footer.svelte?raw'
import fold from '../FoldSection.svelte?raw'

describe('Inspector sections', () => {
  it('Given a step When inspected Then Run when, Timeout and failure are in the Run section', () => {
    // Before: they sat in a "General" section that was shut, and then in a fold at the bottom of one long column.
    expect(inspector).not.toContain('<summary>General</summary>')
    const run = inspector.slice(inspector.indexOf("section === 'run'"), inspector.indexOf("section === 'name'"))
    for (const field of ['Run when (expression)', 'Timeout (e.g. 5s)', 'If this step fails', '<h4>Order']) expect(run).toContain(field)
  })
  // Before: a request step's twelve fields were one column, the ones used most between the ones used least.
  it('Given a request step When inspected Then its fields are in sections under a strip that stays in view', () => {
    const section = (id: string, next: string) => inspector.slice(inspector.indexOf(`section === '${id}'`), inspector.indexOf(`section === '${next}'`))
    expect(inspector).toContain('<Tabs label="Step settings"')
    expect(inspector).toMatch(/\.pin \{[^}]*position: sticky/)
    for (const field of ['With</span>', 'Extra headers']) expect(section('main', 'send')).toContain(field)
    for (const field of ['Version (empty: resolved)', 'Flowi overlay', 'Hooks', 'Open connection', 'Run over connection']) expect(section('send', 'outcomes')).toContain(field)
    expect(section('outcomes', 'run')).toContain('<OutcomesEditor')
    expect(section('name', 'zzz')).toContain('Description')
  })
  it('Given a plain HTTP request When inspected Then the connection fields wait behind a link, and a step with none of a section shows the first', () => {
    expect(inspector).toContain("step.call.ruleKind === 'message'")
    expect(inspector).toContain('Keep a connection open between steps…')
    expect(inspector).toContain("sections.some((x) => x.id === chosen) ? chosen : 'main'")
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
