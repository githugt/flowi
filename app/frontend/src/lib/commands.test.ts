import { describe, expect, it } from 'vitest'
import { commandList, filterCommands, type Command, type PaletteContext } from './commands'

const run = () => {}
const cmd = (id: string, title: string, keys = ''): Command => ({ id, title, keys, icon: '', run })
const all = [cmd('new-request', 'New request', '⌘N'), cmd('new-flow', 'New flow'), cmd('replayer', 'Open Replayer'), cmd('settings', 'Open Settings', '⌘,')]

describe('filterCommands', () => {
  it('lists every command for a blank query', () => {
    expect(filterCommands(all, '  ')).toEqual(all)
  })

  it('keeps the commands that contain every typed word, in any order', () => {
    expect(filterCommands(all, 'flow new').map((c) => c.id)).toEqual(['new-flow'])
    expect(filterCommands(all, 'open').map((c) => c.id)).toEqual(['replayer', 'settings'])
  })

  it('puts a title that starts with the query before one that only contains it', () => {
    expect(filterCommands(all, 'n').map((c) => c.id).slice(0, 2)).toEqual(['new-request', 'new-flow'])
    expect(filterCommands([cmd('a', 'Open Replayer'), cmd('b', 'Replayer help')], 'replayer').map((c) => c.id)).toEqual(['b', 'a'])
  })

  it('is empty when nothing matches', () => {
    expect(filterCommands(all, 'zzz')).toEqual([])
  })
})

describe('commandList', () => {
  const ran: string[] = []
  const ctx = (over: Partial<PaletteContext> = {}): PaletteContext => ({
    environments: ['dev', 'prod'],
    selected: 'dev',
    themes: ['dark', 'light'],
    theme: 'dark',
    canSend: true,
    selectEnvironment: (n) => ran.push('env:' + n),
    setTheme: (n) => ran.push('theme:' + n),
    ...over,
  })
  const titles = (c: PaletteContext) => commandList(() => {}, c).map((x) => x.title)

  it('offers each environment but the one in use, and a way to use none', () => {
    expect(titles(ctx())).toEqual(expect.arrayContaining(['Use environment prod', 'Use no environment']))
    expect(titles(ctx())).not.toContain('Use environment dev')
    expect(titles(ctx({ selected: '' }))).not.toContain('Use no environment')
  })

  it('offers each theme but the one in use', () => {
    expect(titles(ctx())).toContain('Theme: light')
    expect(titles(ctx())).not.toContain('Theme: dark')
  })

  it('offers sending only when a request is open', () => {
    expect(titles(ctx())).toContain('Send the request')
    expect(titles(ctx({ canSend: false }))).not.toContain('Send the request')
  })

  it('runs the chosen environment, theme and send', () => {
    const sent: string[] = []
    const list = commandList((a) => sent.push(a), ctx())
    list.find((c) => c.title === 'Use environment prod')!.run()
    list.find((c) => c.title === 'Theme: light')!.run()
    list.find((c) => c.title === 'Use no environment')!.run()
    list.find((c) => c.title === 'Send the request')!.run()
    expect(ran).toEqual(['env:prod', 'theme:light', 'env:'])
    expect(sent).toContain('send')
  })
})
