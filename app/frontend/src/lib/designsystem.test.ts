import { describe, expect, it } from 'vitest'

// A ratchet: the design system has Button, Menu and the shared overlays, and components still draw their own
// buttons, scrims and z-indexes. The counts below are what exists today, per file. A count may go down (lower the
// number when it does) but not up: new work uses the shared pieces, or says why not by raising a number here.
//
// The order the pieces are migrated in (improvements-table.md, UI7-D5): Button and IconButton first, then Modal,
// Popover and Tabs, then the rest. A file moves to Button when its own `button` rules only repeat what Button
// does; one that restyles buttons on purpose (a swatch, a tree row, a card) is left to its own step.
const components = import.meta.glob('../*.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

// file: [raw <button> elements, `.scrim {` rules, z-index declarations]
const BASELINE: Record<string, [number, number, number]> = {
  'App.svelte': [4, 0, 1],
  'Button.svelte': [1, 0, 0],
  'Canvas.svelte': [4, 0, 2],
  'Chips.svelte': [1, 0, 0],
  'CodeArea.svelte': [1, 0, 0],
  'CodeView.svelte': [2, 0, 0],
  'CommandPalette.svelte': [1, 0, 0],
  'DebugBar.svelte': [1, 0, 0],
  'Dropdown.svelte': [1, 0, 0],
  'ColourPicker.svelte': [2, 0, 0], // the swatches of EnvEditor, moved here to be shared with Manage environments
  'EnvEditor.svelte': [1, 0, 0],
  'FileTree.svelte': [1, 0, 0],
  'FlowEditor.svelte': [1, 0, 0],
  'FlowTools.svelte': [1, 0, 0],
  'FlowiOverlayEditor.svelte': [2, 0, 0],
  'Footer.svelte': [4, 0, 0],
  'HiddenChars.svelte': [1, 0, 1],
  'HelpButton.svelte': [1, 0, 0], // the one "?" button; Hint and ExprHelp draw it
  'HomeScreen.svelte': [1, 0, 0],
  'IconButton.svelte': [1, 0, 0],
  'ImportScreen.svelte': [2, 0, 0],
  'Inspector.svelte': [2, 0, 0],
  'JsonNode.svelte': [2, 0, 0],
  'KVRows.svelte': [2, 0, 0],
  'MainMenuButton.svelte': [1, 0, 0],
  'Menu.svelte': [1, 0, 0],
  'MessageFeed.svelte': [2, 0, 1],
  'MethodPicker.svelte': [3, 0, 1],
  'Modal.svelte': [0, 1, 0],
  'MoveDialog.svelte': [1, 0, 0],
  'OutcomesEditor.svelte': [5, 0, 0],
  'OutputDock.svelte': [2, 0, 0],
  'PasteOffer.svelte': [1, 0, 0],
  'PlaceDialog.svelte': [1, 0, 0],
  'Rail.svelte': [4, 0, 0],
  'RequestEditor.svelte': [7, 0, 0],
  'RequestExtras.svelte': [1, 0, 0],
  'ResponsePane.svelte': [2, 0, 1],
  'RowEditor.svelte': [1, 0, 0],
  'RunMenu.svelte': [3, 0, 0],
  'RunsTab.svelte': [1, 0, 0],
  'Segmented.svelte': [1, 0, 0],
  'SettingsScreen.svelte': [2, 0, 0],
  'Sidebar.svelte': [8, 0, 0],
  'SkipChip.svelte': [1, 0, 0],
  'StepPopup.svelte': [1, 0, 0],
  'Tabs.svelte': [1, 0, 0],
  'TelemetryScreen.svelte': [1, 0, 0],
  'TemplateField.svelte': [1, 0, 2],
  'Toggle.svelte': [1, 0, 0],
  'TrafficLog.svelte': [1, 0, 0],
  'TreeItem.svelte': [1, 0, 0],
  'VarsBar.svelte': [1, 0, 0],
  'VersionPicker.svelte': [1, 0, 0],
  'WelcomeScreen.svelte': [2, 0, 0],
  'XmlNode.svelte': [2, 0, 0],
}

// Rules in a component's own <style> whose selector is `button`, with or without a state: the restyling of what Button does.
const localButtonRules = (text: string): number => {
  const style = /<style[^>]*>([\s\S]*?)<\/style>/.exec(text)?.[1] ?? ''
  return (style.match(/(?:^|[\s,}])button(?::[a-z-]+(?:\([^)]*\))?)*\s*[{,]/gm) ?? []).length
}

const LOCAL_BUTTON_RULES: Record<string, number> = {
  'App.svelte': 1,
  'Button.svelte': 3,
  'Canvas.svelte': 3,
  'Chips.svelte': 1,
  'CodeView.svelte': 2,
  'DebugBar.svelte': 3,
  'EnvEditor.svelte': 3,
  'FlowEditor.svelte': 3,
  'FlowTools.svelte': 3,
  'FlowiOverlayEditor.svelte': 5,
  'Inspector.svelte': 1,
  'JsonNode.svelte': 1,
  'KVRows.svelte': 3,
  'Menu.svelte': 4,
  'MethodPicker.svelte': 1,
  'MoveDialog.svelte': 2,
  'OutcomesEditor.svelte': 2,
  'PlaceDialog.svelte': 2,
  'Rail.svelte': 2,
  'RequestEditor.svelte': 4, // `.addr .send button` sizes Send to the address bar; it was dropped by mistake once and the bar fell apart
  'RequestExtras.svelte': 2,
  'RunMenu.svelte': 4,
  'Segmented.svelte': 6,
  'Sidebar.svelte': 4,
  'Tabs.svelte': 8,
  'TemplateField.svelte': 2,
  'VarsBar.svelte': 2,
  'WelcomeScreen.svelte': 2,
  'XmlNode.svelte': 1,
}

function counts(text: string): [number, number, number] {
  return [
    (text.match(/<button\b/g) ?? []).length,
    (text.match(/\.scrim\s*\{/g) ?? []).length,
    (text.match(/z-index:\s*\d+/g) ?? []).length,
  ]
}

describe('design system ratchet', () => {
  it('lets no component add raw buttons, scrims or z-indexes', () => {
    const over: string[] = []
    for (const [path, text] of Object.entries(components)) {
      const file = path.replace('../', '')
      const have = counts(text)
      const base = BASELINE[file] ?? [0, 0, 0]
      ;['buttons', 'scrims', 'z-indexes'].forEach((what, i) => {
        if (have[i] > base[i]) over.push(`${file}: ${have[i]} ${what}, the baseline is ${base[i]}`)
      })
    }

    expect(Object.keys(components).length).toBeGreaterThan(20)
    expect(over).toEqual([])
  })

  it('lets no component add a button rule of its own', () => {
    // Before: about 50 components restyled `button` in their own <style>, so a disabled, hover or focus look was
    // decided 50 times. A rule counts for each `button` selector (`button {`, `button:hover {`, `button:disabled {`).
    const over: string[] = []
    for (const [path, text] of Object.entries(components)) {
      const file = path.replace('../', '')
      const have = localButtonRules(text)
      const base = LOCAL_BUTTON_RULES[file] ?? 0
      if (have > base) over.push(`${file}: ${have} button rules of its own, the baseline is ${base}`)
    }

    expect(over).toEqual([])
  })

  it('shows every disabled control at the same opacity, from a token', async () => {
    // The test runner leaves a ?raw CSS import empty, and the project has no Node typings.
    // @ts-ignore
    const { readFileSync } = (await import('node:fs')) as { readFileSync: (url: URL, enc: string) => string }
    const appCss = readFileSync(new URL('../app.css', import.meta.url), 'utf8')
    // Before: 0.3, 0.35, 0.4, 0.5, 0.55 and 0.75 were all in use for "disabled". The numbers now live in app.css as
    // --op-disabled (buttons, tabs, toggles), --op-disabled-field (inputs, whose value must stay readable) and
    // --op-busy (a loading button).
    const raw: string[] = []
    for (const [path, text] of Object.entries(components)) {
      for (const m of text.matchAll(/:disabled[^{}]*\{[^}]*?opacity:\s*([.\d]+)/g)) raw.push(`${path.replace('../', '')}: opacity ${m[1]} on :disabled`)
    }

    expect(raw).toEqual([])
    expect(appCss).toMatch(/--op-disabled:\s*0\.4;/)
    expect(appCss).toContain('--op-disabled-field:')
    expect(appCss).toContain('--op-busy:')
    expect(appCss).not.toMatch(/:disabled\)\s*\{\s*opacity:\s*[.\d]/)
  })

  it('has no baseline entry for a file that is gone', () => {
    const files = new Set(Object.keys(components).map((p) => p.replace('../', '')))

    expect(Object.keys(BASELINE).filter((f) => !files.has(f))).toEqual([])
  })
})
