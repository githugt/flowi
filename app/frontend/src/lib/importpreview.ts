// What the import screen lists in its preview: the requests, flows and the rest an import would make, each with
// where it lands and what happens to it if it exists. Go decides the kind, name and status of every file; this only
// groups and words them, so the screen and the tests agree.

export interface PreviewFile {
  rel: string
  exists: boolean
  remove: boolean
  /** request, flow, environment, auth, example, schema, document, proto or other. */
  kind: string
  /** The title of a request or flow, "" for a file that has none. */
  name: string
  /** new, same, differs or remove. */
  status: string
  /** The title of the different request or flow already at this path; absent or "" when none. */
  existingName?: string
  /** What a request shows where an HTTP request shows its method (GET, MQTT, SQS …); absent or "" when none. */
  verb?: string
}

/** What to do with files that already exist: the ids of EXISTING_CHOICES. */
export type ExistingMode = 'skip' | 'overwrite' | 'refuse'

export interface PreviewEntry {
  name: string
  /** The verb pill of a request (GET, MQTT, SQS …), "" for the rest. */
  verb: string
  /** Where it lands in the collection, without the folder of its kind and the extension: "demo/websocket/echo". */
  path: string
  status: string
  /** What the status means with the chosen way to treat existing files. */
  note: string
  /** warn when the entry exists and the way chosen would replace it or stop; ok for a new one; muted for the rest. */
  tone: 'ok' | 'warn' | 'muted'
}

export interface PreviewGroup {
  kind: string
  title: string
  entries: PreviewEntry[]
}

// What a person thinks of as what they import, in the order they are listed; the other kinds are supporting files.
const GROUPS: { kind: string; title: string; folder: string; suffix: string }[] = [
  { kind: 'request', title: 'Requests', folder: 'requests/', suffix: '.req.yaml' },
  { kind: 'flow', title: 'Flows', folder: 'flows/', suffix: '.arazzo.yaml' },
  { kind: 'environment', title: 'Environments', folder: 'environments/', suffix: '.yaml' },
  { kind: 'auth', title: 'Authentications', folder: 'auths/', suffix: '.auth.yaml' },
  { kind: 'example', title: 'Examples', folder: 'requests/', suffix: '.json' },
  { kind: 'proto', title: '.proto files', folder: 'protos/', suffix: '' },
]

/** Where a file lands, as the person names things: its path below the folder of its kind, no extension. */
export function shownPath(f: Pick<PreviewFile, 'rel' | 'kind'>): string {
  const g = GROUPS.find((x) => x.kind === f.kind)
  if (!g) return f.rel
  const below = f.rel.startsWith(g.folder) ? f.rel.slice(g.folder.length) : f.rel
  return g.suffix && below.endsWith(g.suffix) ? below.slice(0, -g.suffix.length) : below
}

/** What happens to a file with this status when files that exist are treated as mode says. */
export function statusNote(status: string, mode: ExistingMode, existingName = ''): { note: string; tone: PreviewEntry['tone'] } {
  // Two operations can share a file name and still be different requests, so say which one is in the way.
  const inWay = existingName ? ` (${'“'}${existingName}${'”'})` : ''
  switch (status) {
    case 'new':
      return { note: 'new', tone: 'ok' }
    case 'same':
      return { note: 'already the same', tone: 'muted' }
    case 'remove':
      return { note: 'stale: removed', tone: 'warn' }
  }
  if (mode === 'overwrite') return { note: `exists${inWay}: will be replaced`, tone: 'warn' }
  if (mode === 'refuse') return { note: `exists${inWay}: stops the import`, tone: 'warn' }
  return { note: `exists${inWay}: yours is kept`, tone: 'warn' }
}

/** The files as lists of requests, flows and so on, each entry worded for the chosen way to treat existing ones. */
export function groupFiles(files: PreviewFile[], mode: ExistingMode): PreviewGroup[] {
  return GROUPS.map((g) => ({
    kind: g.kind,
    title: g.title,
    entries: files
      .filter((f) => f.kind === g.kind)
      .map((f) => ({ name: f.name || shownPath(f), verb: f.verb ?? '', path: shownPath(f), status: f.status, ...statusNote(f.status, mode, f.existingName) })),
  })).filter((g) => g.entries.length > 0)
}

/** The files that are not what the person imports: the kept document, response schemas and the like. */
export function supportingFiles(files: PreviewFile[]): PreviewFile[] {
  return files.filter((f) => !GROUPS.some((g) => g.kind === f.kind))
}

/** The number of requests and flows that would be written (new, or existing and replaced) with this way to treat existing ones. */
export function writtenCount(files: PreviewFile[], mode: ExistingMode): number {
  return files.filter((f) => (f.kind === 'request' || f.kind === 'flow') && (f.status === 'new' || (f.status === 'differs' && mode === 'overwrite'))).length
}

/** The text of the Import button: what it will do, in numbers. */
export function importLabel(files: PreviewFile[], mode: ExistingMode, skipped: number): string {
  const n = writtenCount(files, mode)
  const what = n === 0 ? 'Import' : `Import ${n} ${n === 1 ? 'item' : 'items'}`
  return skipped > 0 ? `${what} (the rest)` : what
}

export interface PreviewStateInput {
  done?: string
  error?: string
  previewed: boolean
  previewing: boolean
  waiting: string
}

/** What the preview area shows: the result of an import, why there is no preview, a preview being read, or the preview. */
export type PreviewState = 'done' | 'failed' | 'ready' | 'reading' | 'waiting'

export function previewState(v: PreviewStateInput): PreviewState {
  if (v.done) return 'done'
  if (v.error) return 'failed'
  if (v.previewed) return 'ready'
  if (v.previewing) return 'reading'
  return 'waiting'
}
