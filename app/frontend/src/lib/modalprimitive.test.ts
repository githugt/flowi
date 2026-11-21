import { describe, expect, it } from 'vitest'
import modalSource from '../Modal.svelte?raw'
import scrimSource from './scrim.ts?raw'
import buttonSource from '../Button.svelte?raw'
import overlays from '../Overlays.svelte?raw'

// The dialogs that moved to Modal; each had its own scrim, box, heading and button row.
const dialogs = import.meta.glob('../{MoveDialog,PlaceDialog,SignInDialog,SpecUpdateDialog,FlowiOverlayWarning,OverlayDialog,FlowiOverlayDialog,BlueprintDialog,VersionDetails,RequestMatrix,FlowiOverlayEditor,RunMenu,PasteOffer,FlowTools,Overlays,CommandPalette}.svelte', { query: '?raw', import: 'default', eager: true }) as Record<string, string>

describe('Modal', () => {
  it('names itself, traps focus and closes on Escape or a press outside, from one place', () => {
    expect(modalSource).toContain('use:modal')
    expect(modalSource).toContain('use:scrimClose')
    expect(modalSource).toContain("role={alert ? 'alertdialog' : 'dialog'}")
    expect(modalSource).toContain('aria-label={label ?? title}')
    expect(modalSource).toContain("e.key === 'Escape'")
  })

  it('has three stacking layers, all from the z-index tokens, and a form variant', () => {
    expect(modalSource).toContain('var(--z-modal)')
    expect(modalSource).toContain('var(--z-modal-above)')
    expect(modalSource).toContain('var(--z-top)')
    expect(modalSource).toContain("onsubmit ? 'form' : 'div'")
  })

  it('is what the sixteen dialogs use, with no scrim, box or button row of their own', () => {
    // Before: each of them wrote `.scrim`, `.dialog` and `.actions` rules and the three attributes of a dialog.
    expect(Object.keys(dialogs)).toHaveLength(16)
    for (const [path, text] of Object.entries(dialogs)) {
      expect(text, path).toContain('<Modal')
      expect(text, path).not.toMatch(/\.scrim\s*\{/)
      expect(text, path).not.toMatch(/\.dialog\s*\{/)
      expect(text, path).not.toContain('use:modal')
    }
  })

  it('has the palette\'s shape: near the top, no heading, no guard on a press outside, and a badge after the heading', () => {
    // Before: the command palette, the flow checks and the prompt and confirm dialogs each drew their own scrim.
    expect(modalSource).toContain("placement?: 'center' | 'top'")
    expect(modalSource).toContain('bare?: boolean')
    expect(modalSource).toContain('{#if !bare}<h3>')
    expect(modalSource).toContain('use:scrimClose={{ onclose: () => onclose?.(), guard }}')
    expect(modalSource).toContain('badge?: Snippet')
    expect(dialogs['../CommandPalette.svelte']).toMatch(/<Modal [^>]*bare[^>]*placement="top"[^>]*guard=\{false\}/)
  })

  it('lets a layer opt out of the dirty-field guard, and gives Button an element ref for focusing Cancel', () => {
    expect(scrimSource).toContain('if (guard && isDirty(before, fields())) return')
    expect(scrimSource).toContain('guard?: boolean')
    expect(buttonSource).toContain('el = $bindable()')
    expect(overlays).toContain('bind:el={cancelButton}')
  })
})
