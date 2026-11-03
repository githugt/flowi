// What the home page works out from Go's counts: which unused-feature ideas to show, and how to say what kinds of
// request there are.

export interface Idea {
  id: string
  title: string
  why: string
  /** "screen:<name>" opens that screen; new-request, new-flow, new-environment and open-request are window actions. */
  action: string
  label: string
}

export interface Home {
  collections: number
  requests: number
  http: number
  messaging: number
  grpc: number
  datasets: number
  flows: number
  environments: number
  secretVars: number
  schedules: number
  blueprints: number
  overlays: number
  withHooks: number
  tracingOn: boolean
  ideas: Idea[]
}

/** The ideas not dismissed, the first `limit` of them, and how many more are waiting behind those. */
export function visibleIdeas(ideas: Idea[], dismissed: Set<string>, limit: number): { shown: Idea[]; more: number } {
  const open = ideas.filter((i) => !dismissed.has(i.id))
  return { shown: open.slice(0, limit), more: Math.max(0, open.length - limit) }
}

/** How many of the ideas on offer were dismissed, so the page can say there are some to bring back. */
export function hiddenCount(ideas: Idea[], dismissed: Set<string>): number {
  return ideas.filter((i) => dismissed.has(i.id)).length
}

/** "12 HTTP · 2 messaging": the kinds of request there are, skipping the kinds there are none of. */
export function breakdownText(h: Pick<Home, 'http' | 'messaging' | 'grpc' | 'datasets'>): string {
  const parts: [number, string][] = [[h.http, 'HTTP'], [h.messaging, 'messaging'], [h.grpc, 'gRPC'], [h.datasets, 'data']]
  return parts.filter(([n]) => n > 0).map(([n, name]) => `${n} ${name}`).join(' · ')
}
