// How many entries each request section holds, for the badge on its tab. Sections that are not lists
// (Body, Auth, Docs…) have no count; they keep the dot that says "has content".
import type { RequestView } from './types'

const size = (rows: readonly unknown[] | null | undefined): number => rows?.length ?? 0
// Rows switched off stay in the file but are not sent, so they are not counted.
const sent = (rows: readonly { Disabled?: boolean }[] | null | undefined): number => rows?.filter((r) => !r.Disabled).length ?? 0

// Headers of an async request travel with each message unless they are the connection's own.
function headerRows(view: RequestView): number {
  if (view.async && !view.async.headersAreConnection) return size(view.async.block.Send?.Headers)
  return sent(view.current.Headers)
}

// A form body counts its fields, so Body reads "2" with two fields and shows the dot for text and files.
function formFields(view: RequestView): number {
  return view.bodyKind === 'form' ? size(view.current.Body?.Form) : 0
}

export function sectionCounts(view: RequestView): Record<string, number> {
  return {
    Params: sent(view.current.Query),
    Headers: headerRows(view),
    Body: formFields(view),
    Variables: size(view.current.Variables),
    Extract: size(view.current.Extract),
    Assert: size(view.current.Assert),
    Hooks: size(view.hookChain),
  }
}

const hasValue = (o: Record<string, unknown> | null | undefined): boolean => !!o && Object.keys(o).length > 0
const waits = (r: { Count: number; Timeout: number; Match: string } | null | undefined): boolean => !!r && (r.Count > 0 || r.Timeout > 0 || r.Match !== '')

/**
 * Whether a section has content, for the green dot on its tab. Before: Receive, Binding, Proto and Settings never
 * showed it, though the Go side already knew (asyncConfigured, grpcConfigured). Credentials is still without one:
 * the window does not hold the values.
 */
export function sectionConfigured(view: RequestView, section: string): boolean {
  const v = view.current
  switch (section) {
    case 'Params': return sent(v.Query) > 0
    case 'Headers': return headerRows(view) > 0
    case 'Body': return view.bodyKind !== 'none'
    case 'Query': return !!view.dataset?.block.Query.trim()
    case 'Values': return !!view.dataset?.block.Params?.length
    case 'Message': return !!(view.async?.block.Send || view.grpc?.block.Body.JSON || view.grpc?.block.Body.Text)
    case 'Receive': return waits(view.async?.block.Receive ?? view.grpc?.block.Receive)
    case 'Binding': return size(view.async?.block.Binding) > 0
    case 'Proto': return size(view.grpc?.block.Proto) > 0 || size(view.grpc?.block.Import) > 0
    case 'Auth': return view.auth.kind !== 'none'
    case 'Hooks': return view.hookChain.length > 0
    case 'Docs': return !!v.Docs
    case 'Variables': return size(v.Variables) > 0
    case 'Settings': return hasValue(v.Settings)
    case 'Extract': return size(v.Extract) > 0
    case 'Assert': return size(v.Assert) > 0
    case 'Outcomes': return size(v.Outcomes) > 0
  }
  return false
}
