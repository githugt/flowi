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
  /** By environment name; a name with the default colour is absent. */
  envColours: Record<string, string>
  selected: string
}

export interface Unit {
  id: string
  kind: 'request' | 'strip' | 'action' | 'container' | 'card'
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
  headH?: number // …and that card's own height (h covers its lanes too)
  card?: boolean // drawn as a card: see Card.face and ports
  ports?: string[] // a card's control ports, top to bottom
}

export interface Edge {
  from: string
  to: string
  curve?: boolean
  needs?: boolean
  into?: boolean
  fail?: boolean
  outcome?: string // the named outcome of the step's request that takes the arrow
  fromStep?: string // the step an after_failure_of arrow names, when its card holds several
  goto?: boolean // a fail edge that is the step's on_error goto, not an arrow written on the step it leads to
  fromPort?: string // the card port it leaves from: done, fail, case-<n>, else
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
  port?: string // the card port that feeds the lane (case-<n> or else)
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

/** One summary row of a card: an icon and a short phrase; it opens its editor in place. */
export interface FaceRow {
  key: string
  icon: string
  summary: string
  editor: string
}

/** A control port: where the flow goes on. tone is done, fail, case or else. */
export interface FacePort {
  id: string
  label: string
  tone: string
  /** The colour the port's outcome was given, and what it was described as. */
  color?: string
  description?: string
}

/** What a step's card shows (see internal/card). */
export interface Face {
  badge: string
  subline: string
  rows: FaceRow[]
  more: FaceRow[]
  ports: FacePort[]
}

export interface Card {
  title: string
  subtitle: string
  hooks?: HookTick[] // a request step: the hooks that run around it
  face?: Face
  color?: string // the colour the person gave the step
  skipped?: boolean // this run leaves the step out
  skipNote?: string // on a step that reads from a skipped one: why it has no value
  break?: boolean // a debug session pauses before this step
}

/** What a value wire can say (see FlowService.WireChoices). */
export interface WireField {
  id: string
  label: string
  needsName?: boolean
}
export interface WireChoices {
  reason?: string
  outputs: string[]
  fields: WireField[]
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
  /** For a request step: the outcomes it has (its own, else its request's), whether they are its own, and the kind its rules are about. */
  outcomes?: { Name: string; When: string; Disabled: boolean; Color?: string; Description?: string }[]
  outcomesOwn?: boolean
  ruleKind?: string
  ref: string
  choices: Choice[]
  version?: string
  /** The flowi overlay laid over the request's version, and the collection's to choose from. */
  overlay?: string
  overlays?: string[]
  /** The connection this step opens and keeps, the one it runs over, and whether it closes that one. */
  open?: string
  over?: string
  close?: boolean
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
  /** What the value is: from the last run, else what that output always is; empty when neither says. */
  type?: string
}

export interface DataEdge {
  from: string
  to: string
  label: string
}

/** One value of a set step: where it comes from (expr, literal, response, generate) and what that source needs. */
export interface ValueRow {
  name: string
  from: string
  value: string
  in: string
  path: string
  header: string
  cookie: string
  regex: string
  seed: string
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
  values?: ValueRow[] // a set step: what it writes
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

/** One thing the step popup offers; mode picks a sibling kind that does the same job another way. */
export interface StepChoice {
  group: string
  name: string
  description: string
  type: string
  mode?: string
}

/** One declared input or output: Value is an input's default and an output's template. */
export interface IOItem {
  name: string
  type: string
  value: string
}

/** A step that reads something a skipped step would have made. */
export interface SkipProblem {
  reader: string
  skipped: string
  text: string
  required: boolean // false: the read has a default
}

/** What the open flow skips this time (kept with the open flow, not in the file). */
export interface SkipView {
  steps: string[]
  hold: boolean // skipped steps keep what the last run gave them
  problems: SkipProblem[]
}

/** The open flow's breakpoints and, while a debug session is paused, where (kept with the open flow, not in the file). */
export interface DebugView {
  breaks: string[]
  paused: string
  vars: { name: string; value: string }[]
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
  /** The variables as the last run ended (masked, one line each); empty before a run. */
  values: { name: string; value: string }[]
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
  kind: string // request, messages or step: which tabs the output dock offers
  size?: string
  messages: { direction: string; text: string; lang?: string }[]
}

export interface HistoryRow {
  label: string
  status: string
  id: string
}

/** Two saved runs compared in depth: the outputs of the steps that differ, line by line, and the variables that ended differently. */
export interface RunComparison {
  steps: Record<string, { kind: 'same' | 'add' | 'del'; text: string }[]>
  vars: { name: string; before: string; after: string; hadBefore: boolean; hadAfter: boolean }[]
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
  skips: SkipView
  debug: DebugView
  start: Face
  end: Face | null
  inputs: IOItem[]
  outputs: IOItem[]
  vars: VarsView
  selected: string
  form: { step?: StepForm; flow: FlowForm }
  palette: PaletteEntry[]
  steps: StepChoice[]
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
  /** The named ways the request can end; null: success and failure. */
  Outcomes?: { Name: string; When: string; Disabled: boolean; Color?: string; Description?: string }[] | null
  Variables: KV[] | null
  Docs: string
  Async?: unknown
  GRPC?: unknown
  Dataset?: unknown
}

export interface ResponseView {
  title: string
  at: string
  /** How long the send took and how big the body was; "" when not known. */
  took: string
  size: string
  passed: boolean
  status: string
  code: string
  sent: PartView[]
  received: PartView[]
  diags: string[]
  bodyJson?: unknown
  /** The body text of an XML response, for the tree and the multi-status list. */
  bodyXml?: string
  phases: { name: string; startMs: number; ms: number }[]
  messages: { direction: string; at: string; body: string; key?: string }[]
  /** The rows of a dataset response, one value per column. */
  table?: TableView
}

/** The rows of a dataset response: a value per column (a number, text, true or false, or null). */
export interface TableView {
  columns: string[]
  rows: unknown[][]
  rowCount: number
  /** The store had more rows than were kept. */
  truncated: boolean
}

/** A flowi overlay as the request screen lists it. */
export interface FlowiOverlayInfo {
  name: string
  description: string
  /** Changes every request using the overlay gets. */
  shared: number
  /** Changes only this request gets, on top. */
  own: number
  used: boolean
}

/** A flowi overlay written as OpenAPI Overlays, one per kept spec it changes. */
export interface SpecExport {
  documents: { spec: string; file: string; yaml: string; requests: string[] }[]
  /** What the OpenAPI Overlays leave out, one sentence each. */
  notCarried: string[]
}

/** What saving the working copy as a flowi overlay would store. */
export interface FlowiOverlayDiff {
  ops: { index: number; summary: string }[]
  /** Parts of the edit an overlay cannot hold. */
  unsupported: string[]
  names: string[]
  /** The overlay laid over the request now. */
  open: string
}

export interface RequestView {
  needsDrop: boolean
  flowiOverlay: string
  flowiOverlays: FlowiOverlayInfo[]
  flowiOverlayNotes: string[]
  /** The header and query rows the chosen overlays set. */
  flowiOverlayMarks: { section: 'header' | 'query'; key: string; overlay: string }[]
  node: string
  name: string
  path: string
  kind: 'http' | 'async' | 'grpc' | 'dataset'
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
  /** The parts of a multipart form body: text, or a file by its path. */
  parts: { name: string; file: boolean; value: string; contentType: string }[]
  /** Set when the JSON body is a JSON-RPC 2.0 call: its method and id (empty for a notification). */
  jsonrpc?: { method: string; id: string }
  multipart: boolean
  auth: { kind: string; value: string; fields: { name: string; value: string; secret: boolean; known: boolean }[]; schemes: { id: string; label: string }[] }
  sending: boolean
  sendError?: string
  response?: ResponseView
  async?: AsyncInfo
  grpc?: GRPCInfo
  dataset?: DatasetInfo
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
  /** How the value is read: "" text, number, boolean or json (an object or list). */
  kind: '' | 'number' | 'boolean' | 'json'
}

export interface EnvView {
  node: string
  name: string
  rows: EnvRow[]
  /** "" for the default. */
  colour: string
  colours: string[]
  /** The environment this one builds on, or "". */
  extends: string
  /** The environments it may build on. */
  bases: string[]
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
  appearance: {
    theme: string
    accent: string
    font: string
    fontSize: number
    /** The palette of the custom theme, when its file reads; customProblem says why it does not. */
    custom?: { base: 'dark' | 'light'; vars: Record<string, string>; rainbow: string[] }
    customProblem?: string
  }
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
  /** The source is text the window handed over: no file to choose. */
  pasted: boolean
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
  form: { addr: string; latency: string; sequence: boolean; cors: boolean; record: string; recordTo: string; recordHar: string; faultErrorPercent: string; faultStatus: string; faultDropPercent: string; faultJitter: string }
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
    /** Shipped with Flowi: new requests start with it unless requests.seed_blueprint names others. */
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
  /** A cloud service: signs in with the provider's own lookup, or the credentials written in the request. */
  cloud: boolean
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
  /** Over gRPC-Web: an HTTP POST to a proxy at an http(s) address; needs .proto files. */
  Web?: boolean
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

/** What a dataset request runs: one read-only query, with its values sent apart from the statement. */
export interface DatasetBlock {
  Driver: string
  Server: string
  Query: string
  Params: string[] | null
  /** The most rows kept; 0 is the default. */
  Limit: number
}

export interface DatasetInfo {
  block: DatasetBlock
  drivers: { id: string; label: string }[]
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
  /** The message hooks run for each message a message request keeps; shown for requests that receive messages. */
  message: string
  othersPre: number
  othersPost: number
  othersMessage: number
  showMessage: boolean
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
  count?: number // how many entries it holds: shown as a badge in place of the dot
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
/** What the footer's Details tab shows for the tab in front. */
export interface DetailsView {
  kind: string // request, flow or ""
  title: string
  rows: { label: string; value: string }[]
}

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

/** The flow and the flows it calls as one standalone Arazzo document. */
export interface StandaloneView {
  text: string
  /** What had no Arazzo equivalent, one sentence each. */
  notes: string[]
  /** The flow has edits the document does not show: it is made from the saved flow. */
  unsaved: boolean
}

// The checks: how much of the API descriptions the flows use, and what a changed description would break.
export interface OperationView {
  collection: string
  request: string
  version: string
  used: string[]
  statuses: { status: number; seen: boolean }[]
  read: string[]
  unread: string[]
}

export interface CoverageView {
  operations: OperationView[]
  operationsUsed: number
  operationsTotal: number
  statusesSeen: number
  statusesTotal: number
  fieldsRead: number
  fieldsTotal: number
}

export interface BreakageView {
  request: string
  version: string
  where: string
  reason: string
  reads: { expr: string; why: string }[]
}

export interface SpecChangeView {
  spec: string
  broken: boolean
  breakages: BreakageView[]
}

// The tools of an open flow beyond Run.
export interface ChecksChoices {
  envs: string[]
  overlays: string[]
  inputs: string[]
  steps: string[]
}

export interface MatrixDimView {
  name: string
  values: string[]
}

export interface MatrixView {
  dims: MatrixDimView[]
  cells: { labels: string[]; status: 'passed' | 'failed' | 'error' | 'cancelled'; why?: string }[]
  passed: number
}

export interface AccessView {
  owner: string
  steps: { id: string; owner: number; attempts: { identity: string; status: number; verdict: 'denied' | 'allowed' | 'unclear' }[] }[]
  exposed: boolean
}

export interface DriftView {
  runStatus: string
  failing: boolean
  requests: {
    request: string
    version: string
    status: number
    noSchema: boolean
    differences: { path: string; kind: 'new' | 'missing' | 'changed'; detail?: string; fails: boolean }[]
  }[]
}

export interface SequenceView {
  title: string
  hosts: string[]
  steps: { kind: 'call' | 'reply' | 'note'; host: number; text: string; failed: boolean }[]
  mermaid: string
}
