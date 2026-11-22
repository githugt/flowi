import { describe, expect, it } from 'vitest'
import { commandList, filterCommands, itemCommands, type Command, type PaletteContext } from './commands'

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

describe('filterCommands with a status and tags', () => {
  const mark = (name: string) => ({ name, color: 'blue', icon: 'code' })
  const item = (id: string, title: string, status?: string, tags: string[] = []): Command => ({ id, title, keys: '', icon: '', run, status: status ? mark(status) : undefined, tags })
  const items = [item('a', 'Login', 'production', ['smoke', 'auth']), item('b', 'Pay', 'review', ['payments']), item('c', 'Health', undefined, ['smoke'])]

  it('Given a tag term When filtering Then only the items with that tag stay, and commands never match', () => {
    // Before: the palette read the title and where an item lives, so a tag or a status could not be searched.
    expect(filterCommands([...items, cmd('x', 'New request')], 'tag:smoke').map((c) => c.id)).toEqual(['a', 'c'])
    expect(filterCommands(items, '#pay').map((c) => c.id)).toEqual(['b'])
  })

  it('Given a status term When filtering Then the item with that status stays', () => {
    expect(filterCommands(items, 'status:rev').map((c) => c.id)).toEqual(['b'])
  })

  it('Given a word that is a status or a tag When filtering Then it matches the item as it does its title', () => {
    expect(filterCommands(items, 'production').map((c) => c.id)).toEqual(['a'])
    expect(filterCommands(items, 'login tag:smoke').map((c) => c.id)).toEqual(['a'])
  })

  it('Given itemCommands When built Then the status and tags of the row come along', () => {
    const roots = [{ id: 'r', kind: 'request', label: 'Login', status: mark('review'), tags: ['smoke'] }] as never
    const [c] = itemCommands(roots, [], () => {})
    expect(c.status?.name).toBe('review')
    expect(c.tags).toEqual(['smoke'])
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

  // Before: nothing hid a credential on screen, so a talk or a screenshot showed the Authorization header whole.
  it('offers to blur secrets, and to show them again once they are', () => {
    const calls: boolean[] = []
    const set = (on: boolean) => calls.push(on)

    const off = commandList(() => {}, ctx({ hideSensitive: false, setHideSensitive: set })).find((x) => x.id === 'hide-sensitive')
    const on = commandList(() => {}, ctx({ hideSensitive: true, setHideSensitive: set })).find((x) => x.id === 'hide-sensitive')
    off?.run()
    on?.run()

    expect(off?.title).toMatch(/Blur secrets/)
    expect(on?.title).toMatch(/Show secrets/)
    expect(calls).toEqual([true, false])
    expect(titles(ctx())).not.toContain('Blur secrets on screen (talk or screenshot mode)')
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

describe('itemCommands', () => {
  const tree = [
    {
      id: 'c:shop', label: 'Shop', kind: 'collection', children: [
        { id: 'f:shop:pets', label: 'Pets', kind: 'folder', children: [{ id: 'r:shop:get', label: 'Get pet', kind: 'request', method: 'GET' }] },
        { id: 'f:shop:checkout', label: 'Checkout', kind: 'flow' },
      ],
    },
    { id: 'e:dev', label: 'dev', kind: 'environment' },
  ]

  it('lists requests, flows and environments, not the folders', () => {
    const got = itemCommands(tree, [], () => {})

    expect(got.map((c) => c.id)).toEqual(['item:r:shop:get', 'item:f:shop:checkout', 'item:e:dev'])
  })

  it('names a request with its method and where it lives', () => {
    const [first] = itemCommands(tree, [], () => {})

    expect(first.title).toBe('GET Get pet')
    expect(first.detail).toBe('Shop › Pets')
  })

  it('puts the open tabs first, in their order', () => {
    const got = itemCommands(tree, ['e:dev', 'f:shop:checkout'], () => {})

    expect(got.map((c) => c.id)).toEqual(['item:e:dev', 'item:f:shop:checkout', 'item:r:shop:get'])
  })

  it('opens the node when run, and is found by where it lives', () => {
    let opened = ''
    const got = itemCommands(tree, [], (n) => (opened = n.id))

    filterCommands(got, 'pets')[0].run()

    expect(opened).toBe('r:shop:get')
  })
})
