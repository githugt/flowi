// Shapes of the views Go publishes (see internal/bridge).
export interface TreeNode {
  id: string
  label: string
  kind: string
  method?: string
  /** What a folder's order calls it: the file or folder name ("" for rows that cannot be reordered). */
  name?: string
  draft?: boolean
  /** Folders and groups: the tree they belong to, requests or flows. Requests drop only into the first. */
  tree?: string
  children?: TreeNode[]
}

export interface Tree {
  name: string
  /** Where workspace.yaml lives; "" when unknown. */
  folder: string
  roots: TreeNode[]
  environments: string[]
  selected: string
}

export interface Unit {
  id: string
  kind: 'request' | 'strip' | 'action' | 'container'
  nodes: string[]
  types: string[]
  path: string
  x: number
  y: number
  w: number
  h: number
  parent?: string
  concurrent?: boolean
  collapsed?: boolean
  core?: number // a request block with attached actions: the index in nodes of the request itself
  cells?: { x: number; y: number; w: number; h: number }[] // …and where each of nodes is drawn, relative to the block
  above?: number // …and how far down its request sits (the stack above it)
  headW?: number // a container drawn as a card with its lanes to the right
}

export interface Edge {
  from: string
  to: string
  curve?: boolean
  needs?: boolean
  into?: boolean
  fail?: boolean
}

export interface Lane {
  container: string
  index: number
  path: string
  x: number
  y: number
  w: number
  h: number
  portY: number
  label: string
}

export interface Diagram {
  units: Unit[]
  edges: Edge[]
  lanes: Lane[]
}

/** One hook that runs around a request step: before it (pre) or after its response (post). */
export interface HookTick {
  phase: 'pre' | 'post'
  level: string
  label: string
  state: string
}

export interface Card {
  title: string
  subtitle: string
  hooks?: HookTick[] // a request step: the hooks that run around it
}

export interface Pair {
  key: string
  value: string
}

export interface Choice {
  id: string
  path: string
  label: string
  dep?: string
}

export interface Param {
  name: string
  kind: string
  doc: string
  required: boolean
  enum?: string[]
  value: string
  complex?: boolean
}

export interface Call {
  kind: 'request' | 'subflow'
  ref: string
  choices: Choice[]
  version?: string
  with: Pair[]
  headers?: Pair[]
  hooks?: string
}

export interface LaneForm {
  label: string
  when?: string
  empty: boolean
}

/** One value that passes between steps: `from` makes it, `to` reads it. */
export interface Use {
  from: string
  to: string
  name: string
}

export interface DataEdge {
  from: string
  to: string
  label: string
}

export interface StepForm {
  id: string
  type: string
  group: string
  summary: string
  name: string
  description: string
  needs: string[]
  previous: string
  needChoices: string[]
  uses: Use[]
  usedBy: Use[]
  when: string
  timeout: string
  concurrent: boolean
  failMode: string
  failTarget: string
  steps: string[]
  attach: string
  attachAt: string
  attachTo: string[]
  attached: string[]
  call?: Call
  params: Param[]
  variables?: Pair[]
  varsHeading?: string
  intro?: string
  cases?: LaneForm[]
  branches?: LaneForm[]
}

export interface FlowForm {
  name: string
  description: string
  preferVersion: string
  inputs: Pair[]
  outputs: Pair[]
  maxConcurrency: string
  rateLimit: string
  onBranchError: string
}

export interface PaletteEntry {
  type: string
  group: string
  summary: string
}

export interface Slot {
  list: string
  index: number
  x: number
  y: number
  h: number
  ok: boolean
  attach?: string // inside a request block: the request the step is drawn with
  side?: string // …and where around it: above, below, or "" beside it
  horizontal?: boolean // …a marker line w wide at (x, y) for a stack above or below the request
  w?: number
}

export interface RunView {
  running: boolean
  error?: string
  summary?: string
  statuses: Record<string, string>
  results: Record<string, string>
}

export interface VarsView {
  set: string[]
  secrets: string[]
  unset: string[]
}

export interface PartView {
  heading: string
  kind: 'lines' | 'table' | 'code'
  lines?: string[]
  rows?: Pair[]
  code?: string
  lang?: string
  collapsed?: boolean
}

export interface TestView {
  name: string
  detail?: string
  passed: boolean
}

export interface NodeData {
  step: string
  status: string
  instances: number
  duration: string
  input: PartView[]
  output: PartView[]
  tests: TestView[]
  diags: string[]
}

export interface HistoryRow {
  label: string
  status: string
  id: string
}

export interface DiffView {
  step: string
  name: string
  before: string
  after: string
  gap: string
  changed: boolean
}

export interface Chip {
  kind: 'error' | 'needs' | 'warning' | 'disk' | 'unsaved'
  text?: string
  steps?: string[]
}

export interface FlowView {
  selection: string[]
  chips: Chip[]
  onDisk: boolean
  needsDrop: boolean
  rev: number
  run: RunView
  vars: VarsView
  selected: string
  form: { step?: StepForm; flow: FlowForm }
  palette: PaletteEntry[]
  node: string
  name: string
  dirty: boolean
  canUndo: boolean
  canRedo: boolean
  error?: string
  diagram: Diagram
  cards: Record<string, Card>
  data: DataEdge[]
}

// Requests. The version keeps the Go field names (model.RequestVersion has no json tags).
export interface KV {
  Key: string
  Value: string
  Description: string
}

export interface Extraction {
  Name: string
  From: string
  Path: string
  Header: string
  Cookie: string
  Regex: string
}

export interface RequestVersion {
  Label: string
  Name?: string
  Method: string
  URL: string
  Query: KV[] | null
  Headers: KV[] | null
  Body: { JSON: string; Text: string; File: string; Form: KV[] | null; Multipart: unknown[] | null }
  Extract: Extraction[] | null
  Assert: string[] | null
  Variables: KV[] | null
  Docs: string
  Async?: unknown
  GRPC?: unknown
}

export interface ResponseView {
  title: string
  at: string
  passed: boolean
  status: string
  code: string
  sent: PartView[]
  received: PartView[]
  diags: string[]
  bodyJson?: unknown
  phases: { name: string; startMs: number; ms: number }[]
  messages: { direction: string; at: string; body: string; key?: string }[]
}

export interface RequestView {
  needsDrop: boolean
  node: string
  name: string
  path: string
  kind: 'http' | 'async' | 'grpc'
  verb: string
  versions: string[]
  versionNames: string[]
  selected: string
  default: string
  replayPath: string
  dirty: boolean
  onDisk: boolean
  error?: string
  current: RequestVersion
  fullUrl: string
  hookChain: string[]
  bodyKind: 'none' | 'text' | 'form' | 'file'
  bodyFormat: 'json' | 'xml' | 'yaml' | 'raw'
  multipart: boolean
  auth: { kind: 'none' | 'bearer' | 'basic' | 'other'; value: string }
  sending: boolean
  sendError?: string
  response?: ResponseView
  async?: AsyncInfo
  grpc?: GRPCInfo
  pipeline: { kind: string; title: string; detail: string; method?: string; tip?: string; state: string; reason?: string; ms: number }[]
  showingHistory: boolean
  inScratch: boolean
  live: { direction: string; at: string; body: string; key?: string }[]
  snippet: { format: string; formats: string[]; exclude: boolean; text: string; error?: string }
}

export interface Prompt {
  title: string
  label: string
  initial: string
  protocols?: Choice[]
}

export interface MenuEntry {
  action: string
  label: string
  prompt?: Prompt
  confirm?: string
  guard?: string
  /** 'move': the window asks where with the Move dialog. */
  dialog?: string
}

/** A place requests can be moved to: a collection, or a request folder in one. */
export interface MoveTarget {
  row: string
  label: string
  depth: number
  draft?: boolean
}

export interface EnvRow {
  name: string
  value: string
  secret: boolean
  secretSet: boolean
  override: boolean
}

export interface EnvView {
  node: string
  name: string
  rows: EnvRow[]
  dirty: boolean
  onDisk: boolean
  needsDrop: boolean
  error?: string
}

export interface LogLine {
  at: string
  level: string
  step: string
  message: string
  tone: string
}

export interface Panels {
  log: { title: string; lines: LogLine[] }[]
  problems: { severity: string; code: string; message: string; where: string; node?: string }[]
  summary: string
  history: { id: number; title: string; at: string; node: string }[]
  /** What opening the workspace found ("" when nothing); its rows are among problems. */
  opening: string
}

export interface SettingView {
  name: string
  description: string
  kind: 'string' | 'bool' | 'int' | 'duration' | 'enum' | 'list'
  enum?: string[]
  value: string
  source: string
  userSet: boolean
  problem?: string
}

export interface SettingsView {
  groups: { name: string; fields: SettingView[] }[]
  appearance: { theme: string; accent: string; font: string; fontSize: number }
}

export interface SecretsView {
  secrets: { environment: string; name: string; state: 'missing' | 'stored' | 'overridden' }[]
  error?: string
}

export interface ManagerView {
  collections: { id: string; name: string; path: string; missing: boolean; draft: boolean; deps: { name: string; repo: string; mounted: boolean }[] }[]
  error?: string
}

export interface ImportView {
  kinds: { id: string; label: string; source: 'file' | 'folder' | 'paste'; overlay: boolean }[]
  kind: string
  file: string
  paste: string
  collections: Choice[]
  collection: string
  previewed: boolean
  summary: string
  files: { rel: string; exists: boolean; remove: boolean }[]
  items: { name: string; skipped: boolean; problems: { severity: string; text: string; hint?: string; where?: string; excerpt?: string[] }[] }[]
  lossy: string[]
  skipped: number
  existing: number
  canImport: boolean
  error?: string
  done?: string
}

export interface ScheduleView {
  name: string
  flow: string
  collection: string
  cron: string
  env: string
  inputs: string
  tz: string
  disabled: boolean
  flowLabel: string
}

export interface SchedulesView {
  runner: { state: 'stopped' | 'starting' | 'running' | 'stopping'; error?: string; notes: string[]; upcoming: { name: string; at: string }[]; triggerAddr: string }
  schedules: ScheduleView[]
  flows: { label: string; collection: string; flow: string }[]
  environments: string[]
  triggerAddr: string
  hasToken: boolean
  error?: string
}

export interface ReplayerView {
  state: 'stopped' | 'starting' | 'running' | 'stopping'
  addr: string
  error?: string
  notes: string[]
  form: { addr: string; latency: string; sequence: boolean; cors: boolean; record: string; recordTo: string }
  targets: { id: string; label: string; path?: string }[]
  traffic: { seq: number; at: string; method: string; path: string; status: number; match: string; request: string; duration: string; remote: string }[]
}

export interface TrashView {
  items: { id: string; kind: 'request' | 'flow'; name: string; path: string; at: string }[]
  after: string
  error?: string
}

export interface BlueprintsView {
  blueprints: {
    key: string
    collection: string
    collectionName: string
    name: string
    path: string
    description: string
    kind: 'flow' | 'post-hooks' | 'pre-hooks'
    params: { name: string; description: string; type: string; default: string }[]
    nodes: number
    /** Shipped with Flow-I: new requests start with it unless requests.seed_blueprint names others. */
    builtIn?: boolean
  }[]
  collections: Choice[]
}

export interface UsedByView {
  rows: { header?: string; text: string; detail?: string; opens: boolean; flow?: string; node?: string; row?: string }[]
  error?: string
}

export interface CookiesView {
  cookies: { key: string; name: string; value: string; domain: string; path: string; expires: string; secure: boolean; httpOnly: boolean }[]
  error?: string
}

export interface AsyncBlock {
  Protocol: string
  Server: string
  Channel: string
  Send: { Body: { JSON: string; Text: string; File: string; Form: KV[] | null; Multipart: unknown[] | null }; Headers: KV[] | null } | null
  Receive: { Count: number; Timeout: number; Match: string } | null
  Binding: KV[] | null
}

export interface AsyncInfo {
  protocols: Choice[]
  modes: string[]
  mode: string
  bindings: string[]
  credentials: string[]
  headersAreConnection: boolean
  block: AsyncBlock
}

export interface GRPCBlock {
  Server: string
  Method: string
  Body: { JSON: string; Text: string }
  Proto: string[] | null
  Import: string[] | null
  Receive: { Count: number; Timeout: number; Match: string } | null
}

/** A method of a gRPC request's schema. */
export interface GRPCMethod {
  full: string
  service: string
  name: string
  clientStream: boolean
  serverStream: boolean
  input: string
  output: string
}

export interface GRPCInfo {
  block: GRPCBlock
  methods: GRPCMethod[]
  methodsError?: string
  streams: boolean
  streamsKnown: boolean
  source: 'files' | 'reflection'
}

export interface HooksView {
  request: string
  pre: string
  post: string
  othersPre: number
  othersPost: number
  dirty: boolean
  blueprints: Choice[]
}

/** One row of a Dropdown. */
export interface DropOption {
  value: string
  label: string
  icon?: string // a name known to Icon
  tone?: string // a verb whose colour the label is drawn in
  dot?: boolean // the "configured" dot
  detail?: string
  separator?: boolean
  disabled?: boolean
}

/** The pop-up that says what an expression field can read. */
export interface ExprHelpView {
  title: string
  intro: string
  names: { name: string; what: string; example: string }[]
  tools: { name: string; what: string; example: string }[]
  generators: { name: string; what: string; example: string }[]
  examples: string[]
}

/** Where the collection of a tab lives (the footer). */
export interface LocationView {
  name: string
  path: string
  branch: string
  scratch: boolean
}

/** A response kept for a request: a saved response or a pinned example. */
export interface StoredView {
  kind: 'saved' | 'example'
  version: string
  name: string
  status: number
  at: string
  size: number
}

/** A variable a request uses that nothing defines yet. name is "" for an unknown {{$generator}}. */
export interface MissingVar {
  expr: string
  name: string
  secret: boolean
}

/** What a send of a request would lack, for the selected environment ("" when none). */
export interface MissingView {
  environment: string
  vars: MissingVar[]
}

/** Environments a mounted collection ships that the workspace does not have yet. */
export interface EnvOffer {
  collection: string
  name: string
  environments: string[]
}

/** The kept spec a request comes from (spec "" for none) and the fields its overlays set. */
export interface SpecOrigin {
  spec: string
  name: string
  collection: string
  fields: { section: 'docs' | 'body' | 'query' | 'header'; key: string; file: string }[]
}

/** What saving a request version to its spec's overlay would write. */
export interface OverlayPreview {
  spec: string
  edits: string
  changes: { what: string; target: string; saved: boolean }[]
  notCarried: { what: string; why: string }[]
}

/** A spec kept in a collection, with its overlays (the edits overlay last). */
export interface KeptSpec {
  name: string
  spec: string
  overlays: string[]
}

/** The preview of Update from spec. */
export interface SpecUpdate {
  spec: string
  added: string[]
  changed: string[]
  removed: string[]
  notes: string[]
}
