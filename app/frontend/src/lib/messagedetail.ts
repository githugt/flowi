// What an opened message in the feed shows: how its body can be read (a JSON tree, plain text, hex), the broker's
// facts about it as chips, and the forms it can be copied in.

export type DetailMode = 'tree' | 'text' | 'hex'

export interface DetailMessage {
  direction: string
  at: string
  body: string
  key?: string
  kind?: string
  type?: string
  partition?: number
  offset?: number
  sequence?: number
  headers?: { key: string; value: string }[]
  size?: number
  hex?: string
}

/** The most bytes of a body the Hex view dumps. A message can be large, and a dump of it is read a screen at a time. */
export const HEX_LIMIT = 2048

/** The body as JSON when it is an object or an array: a lone string or number has no tree to open. */
export function parseTree(body: string): { value: unknown } | null {
  const t = body.trim()
  if (!t.startsWith('{') && !t.startsWith('[')) return null
  try {
    return { value: JSON.parse(t) }
  } catch {
    return null
  }
}

/** The ways this message's body can be read, in the order the switch shows them. Text is always there. */
export function detailModes(m: Pick<DetailMessage, 'body' | 'kind' | 'hex'>): DetailMode[] {
  const out: DetailMode[] = []
  if (m.kind === 'json' && parseTree(m.body)) out.push('tree')
  out.push('text')
  if (m.body !== '' || m.hex) out.push('hex')
  return out
}

/** The mode a message opens in: the tree for JSON, hex for bytes that are not text, else the text. */
export function defaultMode(m: Pick<DetailMessage, 'body' | 'kind' | 'hex'>): DetailMode {
  const modes = detailModes(m)
  if (m.kind === 'binary' && modes.includes('hex')) return 'hex'
  return modes.includes('tree') ? 'tree' : 'text'
}

/**
 * A hex dump, sixteen bytes to a line: offset, the bytes, then the same bytes as text with a dot for what is not
 * printable. It is the layout the bridge uses for a binary body, so both kinds of message look alike.
 */
export function dumpBytes(data: Uint8Array): string {
  const lines: string[] = []
  for (let off = 0; off < data.length; off += 16) {
    const row = data.subarray(off, off + 16)
    let hex = ''
    for (let i = 0; i < 16; i++) {
      hex += i < row.length ? row[i].toString(16).padStart(2, '0') + ' ' : '   '
      if (i === 7) hex += ' '
    }
    const text = Array.from(row, (c) => (c < 0x20 || c > 0x7e ? '.' : String.fromCharCode(c))).join('')
    lines.push(`${off.toString(16).padStart(8, '0')}  ${hex} |${text}|`)
  }
  return lines.join('\n')
}

/** What the Hex view shows for a message, and whether it is only the start. A binary one comes dumped by the bridge. */
export function hexView(m: Pick<DetailMessage, 'body' | 'hex' | 'kind' | 'size'>, limit = HEX_LIMIT): { text: string; shown: number; total: number } {
  if (m.kind === 'binary') {
    const total = m.size ?? 0
    return { text: m.hex ?? '', shown: Math.min(total, limit), total }
  }
  const bytes = new TextEncoder().encode(m.body)
  return { text: dumpBytes(bytes.subarray(0, limit)), shown: Math.min(bytes.length, limit), total: bytes.length }
}

/** The size of the body in bytes: the bridge's for bytes it did not send as text, else the text's own. */
export function bodyBytes(m: Pick<DetailMessage, 'body' | 'size' | 'kind'>): number {
  if (m.kind === 'binary' && m.size !== undefined) return m.size
  return new TextEncoder().encode(m.body).length
}

export interface MetaChip {
  label: string
  value: string
}

/**
 * The facts about a message that are not its body, as label and value chips. Kafka's partition and offset and a
 * JetStream sequence come only when the broker gave them (zero means none). The headers have a table of their own.
 */
export function metaChips(m: DetailMessage): MetaChip[] {
  const out: MetaChip[] = []
  if (m.type) out.push({ label: 'Type', value: m.type })
  if (m.key) out.push({ label: 'Key', value: m.key })
  if (m.offset) {
    out.push({ label: 'Partition', value: String(m.partition ?? 0) })
    out.push({ label: 'Offset', value: String(m.offset) })
  }
  if (m.sequence) out.push({ label: 'Sequence', value: String(m.sequence) })
  return out
}

export type CopyAs = 'body' | 'pretty' | 'message' | 'hex'

/** The forms the message can be copied in. Pretty is for a JSON body, Hex for any body that has bytes to show. */
export function copyChoices(m: Pick<DetailMessage, 'body' | 'kind' | 'hex'>): { value: CopyAs; label: string }[] {
  const out: { value: CopyAs; label: string }[] = [{ value: 'body', label: 'Body' }]
  if (m.kind === 'json' && parseTree(m.body)) out.push({ value: 'pretty', label: 'Body, indented' })
  out.push({ value: 'message', label: 'Whole message as JSON' })
  if (m.body !== '' || m.hex) out.push({ value: 'hex', label: 'Hex dump' })
  return out
}

/** The text copied for a form: what the choice's label says. A binary body's "body" is the note the feed shows for it. */
export function copyText(m: DetailMessage, as: CopyAs): string {
  switch (as) {
    case 'pretty': {
      const t = parseTree(m.body)
      return t ? JSON.stringify(t.value, null, 2) : m.body
    }
    case 'message': {
      const { hex: _hex, ...rest } = m
      return JSON.stringify(rest, null, 2)
    }
    case 'hex':
      return hexView(m, Number.MAX_SAFE_INTEGER).text
    default:
      return m.body
  }
}

/** What the clipboard notice calls it: "the message body", "the hex dump". */
export function copyName(as: CopyAs): string {
  return { body: 'the message body', pretty: 'the indented body', message: 'the message', hex: 'the hex dump' }[as]
}
