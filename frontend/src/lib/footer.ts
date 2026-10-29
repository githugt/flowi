// What the bottom bar says about the open request: its version and the overlays laid over it, as the picker does.
import { pickerLabel } from './versionpicker'

/** "Ver. 2 + vip + slow"; "" when the tab has no version. `overlay` is the stack as saved: "vip+slow". */
export function footerLabel(version: string, overlay: string): string {
  if (!version) return ''
  return 'Ver. ' + pickerLabel(version, overlay ? overlay.split('+') : [])
}
