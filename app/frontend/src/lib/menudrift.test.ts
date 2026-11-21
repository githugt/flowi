import { describe, expect, it } from 'vitest'
import { screens } from './screens'
import { mainItems } from './mainmenu'
import { commandList } from './commands'

async function nativeMenuActions(): Promise<Set<string>> {
  // @ts-ignore
  const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
  const go = readFileSync(new URL('../../../menu.go', import.meta.url), 'utf8')
  return new Set([...go.matchAll(/menuAction\{"[^"]*", "[^"]*", "([a-z-]+)"\}/g)].map((m) => m[1]))
}

describe('screens, menus and the palette', () => {
  const railScreens = screens.filter((s) => s.rail)

  it('opens every tool of the rail from the Linux menu, the native menu and the palette', async () => {
    // Before: Trash was in the rail and the palette but in neither menu.
    const linux = new Set(mainItems.map((i) => i.action))
    const native = await nativeMenuActions()
    const palette = new Set(
      commandList(() => {}, { environments: [], selected: '', themes: [], theme: '', canSend: false, selectEnvironment: () => {}, setTheme: () => {} }).map((c) => c.id),
    )

    expect(railScreens.filter((s) => !linux.has(s.menu)).map((s) => s.menu)).toEqual([])
    expect(railScreens.filter((s) => !native.has(s.menu)).map((s) => s.menu)).toEqual([])
    expect(railScreens.filter((s) => !palette.has(s.menu)).map((s) => s.menu)).toEqual([])
  })

  it('gives the same menu actions to the Linux list as to the native one', async () => {
    const native = await nativeMenuActions()

    expect(mainItems.map((i) => i.action).filter((a) => !native.has(a) && !['docs', 'shortcuts', 'report'].includes(a))).toEqual([])
  })
})
