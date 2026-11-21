/** What of a response the viewer needs to pick how to show the body. */
export interface ShapeInput {
  isDataset: boolean
  hasTable: boolean
  bodyKind: string
  hasHtml: boolean
}

/** Kinds shown by the media viewer: a picture, or a card (with a hex dump) for what has no text form. */
export const mediaKinds = ['image', 'pdf', 'binary']

/**
 * How the body is shown: what the person picked while it still applies to this response, else by what the body is. A
 * dataset's rows are a table, a page is previewed (sandboxed), a picture, PDF or binary body gets the media viewer, and
 * anything else is read as a tree.
 * Before: images, PDFs and binary bodies were drawn as plain text, because the view had no kind for the body.
 */
export function pickShape(chosen: string, o: ShapeInput): string {
  const applies: Record<string, boolean> = {
    rows: o.hasTable,
    preview: o.hasHtml,
    media: mediaKinds.includes(o.bodyKind),
  }
  if (chosen && applies[chosen] !== false) return chosen
  if (o.isDataset && o.hasTable) return 'rows'
  if (o.hasHtml) return 'preview'
  if (mediaKinds.includes(o.bodyKind)) return 'media'
  return 'tree'
}

/** Whether choosing this shape is told to Go (the text formats are); the viewers are drawn in the window alone. */
export const shapeIsFormat = (shape: string): boolean => !['tree', 'status', 'rows', 'preview', 'media'].includes(shape)
