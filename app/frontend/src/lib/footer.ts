// What the bottom bar says about the open request: its version and the overlays laid over it, as the picker does.
import { pickerLabel } from './versionpicker'

/** "Ver. 2 + vip + slow"; "" when the tab has no version. `overlay` is the stack as saved: "vip+slow". */
export function footerLabel(version: string, overlay: string): string {
  if (!version) return ''
  return 'Ver. ' + pickerLabel(version, overlay ? overlay.split('+') : [])
}

/** What the footer chip for the overlays laid over the open request says: how many changes they make, and "identity"
 *  when one of them sets who the request is made as. "" when no overlay is laid over. */
export function overlayChipText(stack: { changes: string[]; identity: boolean }[]): string {
  if (!stack.length) return ''
  const n = stack.reduce((sum, o) => sum + o.changes.length, 0)
  const text = `${n} override${n === 1 ? '' : 's'}`
  return stack.some((o) => o.identity) ? text + ' · identity' : text
}
