// How many entries each request section holds, for the badge on its tab. Sections that are not lists
// (Body, Auth, Docs…) have no count; they keep the dot that says "has content".
import type { RequestView } from './types'

const size = (rows: readonly unknown[] | null | undefined): number => rows?.length ?? 0

// Headers of an async request travel with each message unless they are the connection's own.
function headerRows(view: RequestView): number {
  if (view.async && !view.async.headersAreConnection) return size(view.async.block.Send?.Headers)
  return size(view.current.Headers)
}

// A form body counts its fields, so Body reads "2" with two fields and shows the dot for text and files.
function formFields(view: RequestView): number {
  return view.bodyKind === 'form' ? size(view.current.Body?.Form) : 0
}

export function sectionCounts(view: RequestView): Record<string, number> {
  return {
    Params: size(view.current.Query),
    Headers: headerRows(view),
    Body: formFields(view),
    Variables: size(view.current.Variables),
    Extract: size(view.current.Extract),
    Assert: size(view.current.Assert),
    Hooks: size(view.hookChain),
  }
}
