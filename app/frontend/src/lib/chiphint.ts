import type { VariableHint } from './types'

/** A drawn {{variable}} chip: where it is on the screen and the text it shows. */
export interface ChipBox {
  left: number
  right: number
  top: number
  bottom: number
  /** The chip's text as drawn, braces included. */
  text: string
  /** Drawn as unknown (a name nothing defines). */
  unknown: boolean
}

/** The chip under the point, if any. Before: a chip was only coloured; nothing could be asked of it. */
export function chipAt(chips: ChipBox[], x: number, y: number): ChipBox | undefined {
  return chips.find((c) => x >= c.left && x <= c.right && y >= c.top && y <= c.bottom)
}

/** The expression between a chip's braces: "{{ env.base_url }}" is "env.base_url". An unclosed chip keeps what it has. */
export function exprOfChip(text: string): string {
  return text.replace(/^\{\{/, '').replace(/\}\}$/, '').trim()
}

/** Whether a click on this chip can add the variable somewhere: it is unset and there is an environment to add it to. */
export function canSet(h: VariableHint | null | undefined): h is VariableHint {
  return !!h && !h.known && !!h.environment && !!h.name
}
