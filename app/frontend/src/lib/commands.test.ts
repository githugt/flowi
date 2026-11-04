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
    steps: [{ id: 'login', label: 'Call auth/login' }, { id: 'verify', label: 'Call auth/verify' }],
    variables: ['token', 'subject'],
    selectStep: (id) => ran.push('step:' + id),
    traceValue: (n) => ran.push('trace:' + n),
    compareEnvironments: (a, b) => ran.push(`compare:${a}:${b}`),
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

  it('offers to compare each other environment with the one in use, and nothing with none in use', () => {
    expect(titles(ctx())).toContain('Compare environment prod with dev')
    expect(titles(ctx())).not.toContain('Compare environment dev with dev')
    expect(titles(ctx({ selected: '' })).some((t) => t.startsWith('Compare environment'))).toBe(false)
  })

  it('runs the comparison with the one in use first', () => {
    ran.length = 0

    commandList(() => {}, ctx()).find((c) => c.id === 'compare:prod')!.run()

    expect(ran).toEqual(['compare:dev:prod'])
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
    ran.length = 0
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

describe('flow commands', () => {
  const ran: string[] = []
  const flowCtx = (over: Partial<PaletteContext> = {}): PaletteContext => ({
    environments: [],
    selected: '',
    themes: [],
    theme: '',
    canSend: false,
    steps: [{ id: 'login', label: 'Call auth/login' }, { id: 'verify', label: 'Call auth/verify' }],
    variables: ['token'],
    selectStep: (id) => ran.push('step:' + id),
    traceValue: (n) => ran.push('trace:' + n),
    compareEnvironments: () => {},
    selectEnvironment: () => {},
    setTheme: () => {},
    ...over,
  })
  const titles = (c: PaletteContext) => commandList(() => {}, c).map((x) => x.title)

  it('offers a step of the open flow by its id and what it does, and a trace for each variable it sets', () => {
    expect(titles(flowCtx())).toEqual(expect.arrayContaining(['Go to step: login (Call auth/login)', 'Trace value: token']))
  })

  it('says a step once when its title is just its id', () => {
    const got = titles(flowCtx({ steps: [{ id: 'create', label: 'create' }] }))

    expect(got).toContain('Go to step: create')
  })

  it('offers neither with no flow open', () => {
    const none = titles(flowCtx({ steps: [], variables: [] }))

    expect(none.some((t) => t.startsWith('Go to step') || t.startsWith('Trace value'))).toBe(false)
  })

  it('runs the chosen step and trace', () => {
    ran.length = 0
    const list = commandList(() => {}, flowCtx())

    list.find((c) => c.id === 'step:verify')!.run()
    list.find((c) => c.id === 'trace:token')!.run()

    expect(ran).toEqual(['step:verify', 'trace:token'])
  })

  it('shows only the steps for a query that starts with @, filtered by what follows', () => {
    const list = commandList(() => {}, flowCtx())

    expect(filterCommands(list, '@').map((c) => c.id)).toEqual(['step:login', 'step:verify'])
    expect(filterCommands(list, '@ver').map((c) => c.id)).toEqual(['step:verify'])
  })

  it('shows only the actions, never a step, for a query that starts with >', () => {
    const list = commandList(() => {}, flowCtx())

    const got = filterCommands(list, '>new').map((c) => c.id)

    expect(got).toContain('new-flow')
    expect(got.some((id) => id.startsWith('step:') || id.startsWith('trace:'))).toBe(false)
  })
})
