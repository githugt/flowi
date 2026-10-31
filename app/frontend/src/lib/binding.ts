// The binding list of a message request: the protocol's own options (queue, qos …) and its sign-in values
// (username, password …) share one list in the file, but have two tabs. These helpers keep the two tabs
// from losing or doubling each other's rows, and give the option editor its suggestions.
import type { KV } from './types'

const AUTH = 'auth'

/**
 * The binding list the file holds for what the two tabs show. Each key is written once, the last row
 * winning; an option row named like a sign-in value goes to the credentials (its name is listed in
 * `moved`, so the window can say where it went); the mode of a cloud protocol is written unless it is
 * the default.
 */
export function mergeBinding(options: KV[], mode: string, creds: KV[], credentialNames: string[]): { list: KV[]; moved: string[] } {
  const moved: string[] = []
  const plain: KV[] = []
  const signIn = new Map<string, KV>(creds.map((c) => [c.Key, c]))
  for (const row of options) {
    if (credentialNames.includes(row.Key)) {
      signIn.set(row.Key, row)
      if (!moved.includes(row.Key)) moved.push(row.Key)
    } else {
      plain.push(row)
    }
  }
  const list = [...lastWins(plain)]
  if (mode && mode !== 'default') list.push({ Key: AUTH, Value: mode, Description: '' })
  list.push(...signIn.values())
  return { list, moved }
}

function lastWins(rows: KV[]): KV[] {
  const byKey = new Map<string, KV>()
  for (const r of rows) {
    byKey.delete(r.Key) // the later row takes the later place too
    byKey.set(r.Key, r)
  }
  return [...byKey.values()]
}

/** The option names to suggest: the protocol's names without its sign-in values, which have their own tab. */
export const optionNames = (names: string[], credentialNames: string[]): string[] => names.filter((n) => !credentialNames.includes(n))

// Values a transport reads from a fixed set. Checked against the transports in flowi-lib/transport.
const VALUES: Record<string, string[]> = {
  qos: ['0', '1', '2'],
  retain: ['true', 'false'],
  version: ['5', '3.1.1', '3.1'], // MQTT
  offset: ['latest', 'earliest'], // Kafka
  'sasl.mechanism': ['PLAIN', 'SCRAM-SHA-256', 'SCRAM-SHA-512'],
  deliver: ['new', 'all', 'last'], // NATS JetStream
  mode: ['pubsub', 'stream'], // Redis
  read: ['peek', 'lock'], // Service Bus
}

/** The values option `key` takes when the set is fixed; none for free text. */
export const bindingValues = (key: string): string[] => VALUES[key] ?? []

export interface BulkSuggestion {
  kind: 'key' | 'value'
  items: string[]
}

// The line the caret is in: where it starts, its text up to the caret, and where its colon is (-1 if none yet).
function lineAt(text: string, caret: number) {
  const start = text.lastIndexOf('\n', caret - 1) + 1
  const before = text.slice(start, caret)
  return { start, before, colon: before.indexOf(':') }
}

/**
 * What to offer at the caret of the bulk text: names before the colon (those that start with what is
 * typed and are not used on another line), values after it when the option's set is fixed. Null when
 * there is nothing to offer.
 */
export function bulkSuggestions(text: string, caret: number, names: string[]): BulkSuggestion | null {
  const { start, before, colon } = lineAt(text, caret)
  if (colon < 0) {
    const current = text.slice(0, start).split('\n').length - 1
    const used = new Set(
      text
        .split('\n')
        .filter((_, i) => i !== current)
        .map((l) => l.split(':')[0].trim()),
    )
    const typed = before.trim().toLowerCase()
    const items = names.filter((n) => n.toLowerCase().startsWith(typed) && !used.has(n))
    return items.length ? { kind: 'key', items } : null
  }
  const key = before.slice(0, colon).trim()
  const typed = before.slice(colon + 1).trim().toLowerCase()
  const items = bindingValues(key).filter((v) => v.toLowerCase().startsWith(typed) && v.toLowerCase() !== typed)
  return items.length ? { kind: 'value', items } : null
}

/** The text and caret after the person picks `item`: a name replaces what was typed and adds ": ", a value replaces the rest of the line. */
export function completeBulk(text: string, caret: number, kind: 'key' | 'value', item: string): { text: string; caret: number } {
  const { start, before, colon } = lineAt(text, caret)
  const lineEnd = text.indexOf('\n', caret) < 0 ? text.length : text.indexOf('\n', caret)
  const rest = text.slice(lineEnd)
  const head = kind === 'key' ? `${item}: ` : `${before.slice(0, colon + 1)} ${item}`
  return { text: text.slice(0, start) + head + rest, caret: start + head.length }
}
