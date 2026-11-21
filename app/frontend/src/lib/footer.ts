// What the bottom bar says about the open request: its version and the overlays laid over it, as the picker does.
import { pickerLabel } from './versionpicker'

/** "Request ver. 2 + vip + slow"; "" when the tab has no version. `overlay` is the stack as saved: "vip+slow". */
export function footerLabel(version: string, overlay: string): string {
  if (!version) return ''
  // Before: "Ver. 2" sat next to the app's "v1.4", and nothing said which was which.
  return 'Request ver. ' + pickerLabel(version, overlay ? overlay.split('+') : [])
}
