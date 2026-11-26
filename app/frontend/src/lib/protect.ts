const SAFE = new Set(['GET', 'HEAD', 'OPTIONS'])

/** What a request does, as far as the confirmation cares. */
export interface SendShape {
  kind: string // http, async, grpc or dataset
  method?: string // an HTTP request's method
  writes?: boolean // a dataset query that may change data
  /** A GraphQL body: the kind of operation that runs ('' when it cannot be told). Undefined for any other body. */
  graphql?: string
}

/** Whether sending a request changes data: any HTTP method but GET, HEAD and OPTIONS (a GraphQL query is not one, whatever its method); any message or gRPC call; a query allowed to write. */
export function sendWrites(r: SendShape): boolean {
  switch (r.kind) {
    case 'http':
      // A GraphQL query goes out with POST and still only reads; a mutation, or a document that cannot be told apart, writes.
      if (r.graphql !== undefined) {
        const method = (r.method ?? 'POST').toUpperCase()
        if (method !== 'POST' && method !== 'GET') return true
        return r.graphql !== 'query' && r.graphql !== 'subscription'
      }
      return !SAFE.has((r.method ?? 'GET').toUpperCase())
    case 'dataset':
      return !!r.writes
    default:
      return true
  }
}

/**
 * Whether the app asks before this send: the environment is protected and the request writes.
 * Before: production was two clicks away and nothing asked.
 */
export function sendNeedsConfirm(env: string, protectedEnvs: readonly string[], r: SendShape): boolean {
  return !!env && protectedEnvs.includes(env) && sendWrites(r)
}

/** Whether the app asks before a flow runs: the environment is protected (a flow may send anything). */
export function runNeedsConfirm(env: string, protectedEnvs: readonly string[]): boolean {
  return !!env && protectedEnvs.includes(env)
}

/** How many writing steps the run question lists before "and N more". */
export const WRITES_SHOWN = 6

/**
 * The question before a flow runs in a protected environment: the steps that change data, the first few by name.
 * Before: it said only that "a flow can send requests that change data", whether or not this one did.
 */
export function writesQuestion(env: string, lines: readonly string[]): string {
  const shown = lines.slice(0, WRITES_SHOWN)
  const more = lines.length - shown.length
  const head = `${env} is a protected environment. This flow changes data in ${lines.length} ${lines.length === 1 ? 'step' : 'steps'}:`
  return [head, ...shown.map((l) => `• ${l}`), ...(more > 0 ? [`and ${more} more`] : [])].join('\n')
}

/** The question when the list of writing steps could not be read: the old warning, so a failed check never lets a run through unasked. */
export function writesUnknownQuestion(env: string): string {
  return `${env} is a protected environment. A flow can send requests that change data, and this one could not be checked.`
}
