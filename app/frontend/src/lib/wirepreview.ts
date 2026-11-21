import type { WirePreview } from './types'

/** What the wire will write: the template that reads the step's value (Go's readRef). */
export function wireRef(from: string, take: string): string {
  const path = take.trim()
  if (!from) return `{{inputs.${path}}}`
  return `{{nodes.${from}${path ? '.' + path : ''}}}`
}

/** The line under "Take": what the path gives in the step's last run, or why there is nothing to show. */
export function previewLine(p: WirePreview | null): { kind: 'none' | 'value' | 'missing' | 'norun'; text: string } {
  if (!p) return { kind: 'none', text: '' }
  if (!p.ran) return { kind: 'norun', text: 'Run the flow to see the value this would pass.' }
  if (!p.found) return { kind: 'missing', text: 'Nothing at this path in the last run.' }
  return { kind: 'value', text: p.text === '' ? '(empty text)' : p.text }
}
