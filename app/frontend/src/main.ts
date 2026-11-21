import { mount } from 'svelte'
import './app.css'
import App from './App.svelte'
import { reportUnhandled, frontendReport, mayReport } from './lib/errors'
import { SystemService } from '../bindings/github.com/githugt/flowi/app/internal/bridge'
import { ui } from './lib/ui.svelte'
import { dropPointerFocusOnBlur } from './lib/focusring'
import { installDropGuard } from './lib/dropguard'
import { busy } from './lib/busy.svelte'
import { keepsNativeMenu } from './lib/nativemenu'

// Calls to Go that nobody awaited still show their errors.
window.addEventListener('unhandledrejection', (e) => reportUnhandled(e.reason, (text, level) => ui.toast(text, level)))

// What the window catches also goes to the app's log, a few reports at most, so a crash leaves a trace.
let reported = 0
const toLog = (kind: 'error' | 'rejection', reason: unknown) => {
  const r = frontendReport(kind, reason)
  if (!r || !mayReport(reported)) return
  reported++
  void Promise.resolve(SystemService.ReportFrontendError(r.kind, r.message, r.stack)).catch(() => {})
}
window.addEventListener('unhandledrejection', (e) => toLog('rejection', e.reason))
window.addEventListener('error', (e) => toLog('error', e.error ?? e.message))

// A right-click that nothing in the app answers does not open the web view's own menu (Reload, Inspect Element). A field
// and a text selection keep it, for Copy and Paste.
window.addEventListener('contextmenu', (e) => {
  if (!keepsNativeMenu(e.target)) e.preventDefault()
})

// A clicked control does not come back from Alt+Tab wearing a focus ring.
dropPointerFocusOnBlur()

// A stray file dropped on the window does not take the webview away from the app.
installDropGuard(window)

// A call to Go that waits shows a thin bar, and after five seconds a notice (Busy.svelte).
busy.install(window)

mount(App, { target: document.getElementById('app')! })
