// The line under a setting that says what it accepts and what it was before: the range of a number, an example of a
// duration, and the default once the value differs from it.
// Before: values were free text with no hint of the bounds, and a changed value never showed the default.
import type { SettingView } from './types'

/** What the field accepts, or '' when the description says it all. */
export function acceptsText(f: Pick<SettingView, 'kind'> & { min?: number; max?: number }): string {
  if (f.kind === 'duration') return 'A duration, like 500ms, 30s or 2m.'
  if (f.kind === 'int') {
    const lo = f.min ?? 0
    const hi = f.max ?? 0
    if (hi > 0) return `A whole number from ${lo} to ${hi}.`
    return lo > 0 ? `A whole number, ${lo} or more.` : 'A whole number.'
  }
  return ''
}

/** "Default: 30s" when the person has changed the value and the default is known; '' otherwise. */
export function defaultNote(f: Pick<SettingView, 'userSet' | 'value'> & { default?: string }): string {
  if (!f.userSet || f.default === undefined || f.default === '' || f.value === f.default) return ''
  return `Default: ${f.default}`
}
