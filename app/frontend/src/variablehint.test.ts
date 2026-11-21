import { describe, expect, it } from 'vitest'

describe('Variable chips in a template field', () => {
  it('should ask the bridge about the chip under the mouse and add an unset one to the environment on a click', async () => {
    // Before: a chip was only coloured; its value was in another screen and an unset name had to be typed there.
    const field = (await import('./TemplateField.svelte?raw')).default as string

    expect(field).toContain('RequestService.VariableHint(exprOfChip(chip.text))')
    expect(field).toContain('onmousemove={overChip}')
    expect(field).toContain('onmouseleave={leaveChip}')
    expect(field).toContain('app.addEnvironmentRows(tip.hint.environment')
    expect(field).toContain('role="tooltip"')
  })

  it('should draw the tip in theme tokens only', async () => {
    const field = (await import('./TemplateField.svelte?raw')).default as string
    const rule = field.match(/\.tip \{[^}]*\}/)?.[0] ?? ''

    expect(rule).toContain('var(--raised)')
    expect(rule).not.toMatch(/#[0-9a-f]{3,6}\b|rgb\(/i)
  })
})
