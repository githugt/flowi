import { describe, expect, it } from 'vitest'
import menu from '../Menu.svelte?raw'

const callers = import.meta.glob('../{EnvEditor,MainMenuButton,Overlays,Sidebar,FlowTools,RunMenu,VersionPicker,RequestEditor,Dropdown}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Menu on a layer, like Popover', () => {
  it('closes on a press outside, on Escape (handing the focus back) and on Tab, and passes a right-click on', () => {
    expect(menu).toContain('onpointerdown={onclose}')
    expect(menu).toContain("e.key === 'Escape'")
    expect(menu).toContain("e.key === 'Tab'")
    expect(menu).toContain('opener.focus')
    expect(menu).toContain("dispatchEvent(new MouseEvent('contextmenu'")
  })

  it('is placed by a point and kept inside the window, on the menu layers', () => {
    expect(menu).toContain('menuPlace(at')
    expect(menu).toContain('var(--z-menu)')
    expect(menu).not.toMatch(/z-index:\s*\d/)
    expect(menu).not.toContain('class: cls')
  })

  it('is opened with `at` by its callers, which no longer position it or close it from a window pointer handler', () => {
    // Before: each caller wrote CSS like `top: calc(100% + 4px); right: 0` for it, made a wrapper `position: relative`,
    // stopped the button's pointer-down and closed the menu from a window pointer-down handler.
    expect(Object.keys(callers)).toHaveLength(9)
    for (const [path, text] of Object.entries(callers)) {
      expect(text, path).toMatch(/<Menu [^>]*(\bat=|\{at\})/)
      expect(text, path).not.toMatch(/role="menu"/)
      expect(text, path).not.toMatch(/<Menu [^>]*\b(class|style)=/)
      expect(text, path).not.toMatch(/<svelte:window[^>]*onpointerdown/)
      expect(text, path).not.toMatch(/:global\(\.(sharemenu|mainmenu|wsmenu|moremenu)\)/)
    }
  })

  it('has captions and toggles that keep it open, for the pickers that group their items', () => {
    // Before: the version picker and the send menu drew their own captions ("Versions", "When sending") and ticks,
    // and the picker's overlay toggles kept their menu open by stopping the pointer-down of the whole menu.
    expect(menu).toContain('heading?: string')
    expect(menu).toContain('multi?: boolean')
    expect(menu).toContain('if (!item.multi) onclose()')
    expect(menu).toContain('menuitemcheckbox')
    expect(callers['../VersionPicker.svelte']).toContain("multi: g === 'Overlays'")
    expect(callers['../VersionPicker.svelte']).toContain('heading: n === 0 ? g : undefined')
    expect(callers['../RequestEditor.svelte']).toContain("heading: i === 0 ? 'When sending'")
  })
})
