import { describe, expect, it } from 'vitest'
import { groupMenuItems, groupName, inDefault, toggledGroups } from './envmanage'
import { envTools } from './envtools'
import { environmentNode } from './treewalk'
import type { TreeNode } from './types'

describe('toggledGroups', () => {
  it('should switch a group on or off and keep the names sorted', () => {
    expect(toggledGroups(['payments'], 'checkout')).toEqual(['checkout', 'payments'])
    expect(toggledGroups(['checkout', 'payments'], 'checkout')).toEqual(['payments'])
    expect(toggledGroups(['a'], 'a')).toEqual([])
  })

  // Before: choosing the default group took the environment out of every named one, and choosing a named group took it
  // out of the default group; the choice is the person's.
  it('should keep the default group when a named group is added to an environment that has none', () => {
    expect(toggledGroups([], 'a')).toEqual(['a', 'default'])
  })

  it('should add the default group beside the named ones and take it off again without touching them', () => {
    expect(toggledGroups(['checkout'], '')).toEqual(['checkout', 'default'])
    expect(toggledGroups(['checkout', 'default'], '')).toEqual(['checkout'])
    expect(toggledGroups(['checkout', 'payments', 'default'], '')).toEqual(['checkout', 'payments'])
  })

  it('should fall back to the default group alone when nothing else is left, and write that as no entry', () => {
    expect(toggledGroups(['checkout', 'default'], 'checkout')).toEqual([])
    expect(toggledGroups(['checkout'], 'checkout')).toEqual([])
    expect(toggledGroups([], '')).toEqual([])
  })

  it('should leave the list it was given as it was', () => {
    const before = ['b']

    toggledGroups(before, 'a')

    expect(before).toEqual(['b'])
  })
})

describe('inDefault and groupName', () => {
  it('should read no entries and the default entry as the default group', () => {
    expect(inDefault([])).toBe(true)
    expect(inDefault(['a', 'default'])).toBe(true)
    expect(inDefault(['a'])).toBe(false)
  })

  it('should call the default entry Default and keep the other names', () => {
    expect(groupName('default')).toBe('Default')
    expect(groupName('payments')).toBe('payments')
  })
})

describe('groupMenuItems', () => {
  const groups = [
    { name: '', label: 'Default', count: 1 },
    { name: 'checkout', label: 'checkout', count: 2 },
    { name: 'pay', label: 'pay', count: 1 },
  ]

  // Before: the default group was missing from the menu, so an environment could be put in named groups but not back.
  it('should list the default group first, tick the groups the environment is in and keep the menu open on a tick', () => {
    const items = groupMenuItems(groups, ['pay'], () => {}, () => {})

    expect(items.map((i) => i.label)).toEqual(['Default', 'checkout', 'pay', 'New group…'])
    expect(items.map((i) => i.checked)).toEqual([false, false, true, undefined])
    expect(items.slice(0, 3).every((i) => i.multi)).toBe(true)
    expect(items[3].multi).toBeFalsy()
    expect(items[3].separated).toBe(true)
  })

  it('should tick the default group when the environment has no group or the default entry, beside the named ones', () => {
    expect(groupMenuItems(groups, [], () => {}, () => {})[0].checked).toBe(true)
    expect(groupMenuItems(groups, ['pay'], () => {}, () => {})[0].checked).toBe(false)
    expect(groupMenuItems(groups, ['pay', 'default'], () => {}, () => {}).map((i) => i.checked)).toEqual([true, false, true, undefined])
  })

  it('should call toggle with the group name and create for the last item', () => {
    const toggled: string[] = []
    let created = 0
    const items = groupMenuItems(groups, [], (g) => toggled.push(g), () => created++)

    items[0].onselect()
    items[2].onselect()
    items[3].onselect()

    expect(toggled).toEqual(['', 'pay'])
    expect(created).toBe(1)
  })

  it('should offer Default and New group… when there is no named group', () => {
    const items = groupMenuItems([{ name: '', label: 'Default', count: 0 }], [], () => {}, () => {})

    expect(items.map((i) => i.label)).toEqual(['Default', 'New group…'])
    expect(items[0].checked).toBe(true)
  })
})

describe('environmentNode', () => {
  it('should find an environment row at any depth by its name', () => {
    const roots: TreeNode[] = [{ id: 'c:1', label: 'Shop', kind: 'collection', children: [{ id: 'e:1', label: 'staging', kind: 'environment' }, { id: 'f:1', label: 'staging', kind: 'folder' }] }]

    expect(environmentNode(roots, 'staging')?.id).toBe('e:1')
    expect(environmentNode(roots, 'nope')).toBeUndefined()
    expect(environmentNode(undefined, 'staging')).toBeUndefined()
  })
})

describe('the Manage environments screen', () => {
  it('should be opened from the menus and the palette and drawn by the app', async () => {
    const { screens } = await import('./screens')
    const { mainItems } = await import('./mainmenu')
    const app = (await import('../App.svelte?raw')).default as string
    const screen = (await import('../EnvManageScreen.svelte?raw')).default as string

    expect(screens.some((s) => s.id === 'screen:envmanage' && s.menu === 'envmanage' && !s.rail)).toBe(true)
    expect(mainItems.some((i) => i.action === 'envmanage')).toBe(true)
    expect(app).toContain("tab.node === 'screen:envmanage'")
    expect(screen).toContain('EnvironmentService.Manage()')
    expect(screen).toContain('EnvironmentService.MoveToGroups')
    expect(screen).toContain('EnvironmentService.ColourEnvironments')
    expect(screen).toContain('<ColourPicker')
    expect(screen).toContain('EnvironmentService.RenameGroup')
    expect(screen).toContain('EnvironmentService.DeleteGroup')
    expect(screen).toContain('onclick={newGroup}')
    expect(screen).toContain('EnvironmentService.AddGroup')
  })

  it('should keep its launchpad cards to screens and actions that exist, one per id', async () => {
    const { screens } = await import('./screens')
    const src = (await import('./envtools?raw')).default as string
    const opened = [...src.matchAll(/openScreen\('(screen:[a-z]+)'/g)].map((m) => m[1])

    expect(new Set(envTools.map((t) => t.id)).size).toBe(envTools.length)
    for (const id of opened) expect(screens.some((s) => s.id === id), id).toBe(true)
    for (const t of envTools) expect(t.title && t.hint && t.icon, t.id).toBeTruthy()
  })
})
