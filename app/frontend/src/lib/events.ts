// The events Go sends to the window, named once.
// Before: each Events.On('…') spelled the name as a bare string, so a renamed or mistyped event listened to nothing and
// nothing said so. A test compares this list with the names Go emits.
import { Events } from '@wailsio/runtime'

export const eventNames = [
  'authprompt', 'auths', 'blueprints', 'clipboard', 'close', 'deleted', 'environment', 'flow', 'flowhistory', 'flowrun', 'hooks',
  'import', 'manager', 'matrix', 'menu', 'notice', 'open', 'panels', 'passphrase', 'quit-requested', 'reorder-needs-confirm',
  'replayer', 'request', 'schedules', 'secrets', 'settings', 'sidebar', 'spec-origin', 'telemetry', 'tracing', 'trash', 'welcome', 'workspace',
] as const

export type EventName = (typeof eventNames)[number]

/** Listens to one of Go's events; returns the function that stops listening. */
export function onEvent(name: EventName, handler: (data: any) => void): () => void {
  return Events.On(name, (e: any) => handler(e?.data))
}
