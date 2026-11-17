// What the replayer will do with a call, worked out from the form, for the preview on the replayer screen.
import type { ForwardMode, ForwardPreview } from './types'

export interface ForwardForm { forward: ForwardMode; forwardTo: string; forwardOnly: string; recordTo: string; recordHar: string }

export type Line = { text: string; tone: 'ok' | 'warn' | 'bad' | 'plain' }

// The comma separated list of APIs, without blanks or a trailing slash.
export const splitList = (text: string): string[] => text.split(',').map((s) => s.trim()).filter(Boolean)

const trimSlash = (s: string) => s.replace(/\/+$/, '')

// Whether an API (base) falls inside an allowed one: same scheme and host, and the allowed path is a prefix.
export function within(base: string, allowed: string): boolean {
  const a = trimSlash(base), b = trimSlash(allowed)
  return a === b || a.startsWith(b + '/')
}

const modeName = (m: ForwardMode) => (m === 'save' ? 'pass through and save' : m === 'pass' ? 'pass through' : 'replay only')

// A short line for the replay bar.
export function forwardLabel(f: { forward: ForwardMode; forwardTo: string; forwardOnly: string }): string {
  if (!f.forward) return 'listening'
  const verb = f.forward === 'save' ? 'saving calls with no saved answer from ' : 'passing calls with no saved answer to '
  const to = f.forwardTo.trim()
  if (to) return verb + to
  const only = splitList(f.forwardOnly)
  return verb + (only.length ? only.join(', ') : 'their own APIs')
}

// The lines of the preview, in the order a call meets them.
export function forwardPreview(form: ForwardForm, p: ForwardPreview | null, labelOf: (id: string) => string = (s) => s): Line[] {
  const out: Line[] = [{ text: 'A call a saved response can answer is replayed from that response. It never reaches a real API.', tone: 'ok' }]
  const har = form.recordHar.trim() ? { text: `Every call the replayer handles is also written to ${form.recordHar.trim()}.`, tone: 'plain' as const } : null
  if (!form.forward) {
    out.push({ text: 'Any other call is answered "not found" (404). Nothing is forwarded or saved.', tone: 'plain' })
    if (har) out.push(har)
    return out
  }
  const to = form.forwardTo.trim()
  const only = splitList(form.forwardOnly)
  const save = form.forward === 'save'
  const env = p?.env ? ` (environment ${p.env})` : ''
  out.push({ text: `Mode: ${modeName(form.forward)}.`, tone: 'plain' })
  if (to) {
    out.push({ text: `Every call with no saved answer is sent to ${to}${save ? ' and its answer is saved' : ''}.`, tone: 'ok' })
    if (only.length) out.push({ text: '"Only these APIs" is ignored while one API is set.', tone: 'warn' })
    if (save) {
      out.push({ text: `A call matching no request becomes a new request in ${labelOf(form.recordTo) || 'the Scratch draft'}.`, tone: 'plain' })
    } else {
      out.push({ text: 'Answers are relayed; nothing is saved, so stopping the replayer returns those calls to "not found".', tone: 'plain' })
    }
  } else {
    out.push({ text: `A call with no saved answer goes to the API of the saved request it matches${env}${only.length ? ', if that API is in the list below' : ''}${save ? '; its answer is saved on that request' : ''}.`, tone: 'ok' })
    out.push({ text: 'A call that matches no request at all has nowhere to go and is answered "not found". Set one API to send those somewhere.', tone: 'warn' })
    const targets = p?.targets ?? []
    if (!targets.length) out.push({ text: 'No saved request has a base URL to forward to.', tone: 'warn' })
    for (const t of targets) {
      const n = `${t.requests} request${t.requests === 1 ? '' : 's'}`
      if (t.problem) out.push({ text: `${t.base} (${n}): ${t.problem}`, tone: 'bad' })
      else if (only.length && !only.some((o) => within(t.base, o))) out.push({ text: `${t.base} (${n}): not in the list, so calls stay "not found".`, tone: 'warn' })
      else out.push({ text: `${t.base} (${n}): ${save ? 'forwarded and saved' : 'forwarded'}.`, tone: 'ok' })
    }
    for (const o of only) {
      if (!targets.some((t) => within(t.base, o))) out.push({ text: `${o}: no saved request calls this API.`, tone: 'warn' })
    }
  }
  if (har) out.push(har)
  return out
}
