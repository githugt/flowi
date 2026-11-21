// The name a section tab shows for each kind of request.
// Before: a gRPC request's "Headers" tab said Headers and carried the HTTP hint, though gRPC calls them metadata.

/** The tab title for a section of a request of this kind. */
export function sectionTitle(kind: string, section: string): string {
  if (kind !== 'grpc') return section
  return ({ Message: 'Body', Receive: 'Stream', Headers: 'Metadata' } as Record<string, string>)[section] ?? section
}

/** What the rows of the Headers section are called for this kind: "header" or, for gRPC, "metadata". */
export function headerWord(kind: string): 'header' | 'metadata' {
  return kind === 'grpc' ? 'metadata' : 'header'
}
