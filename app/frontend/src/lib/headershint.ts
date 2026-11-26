// The hint under an empty header list. Before, a WebSocket or SSE request said "Credentials go in Auth" although
// those requests have no Auth tab: the user must type the Authorization header there.
export type HeaderListKind = 'http' | 'connect' | 'send'

export function headersHint(kind: HeaderListKind): string {
  switch (kind) {
    case 'send':
      return 'Headers travel with each message sent.'
    case 'connect':
      return 'Sent when the connection opens. Credentials go in Auth.'
    default:
      return 'Content-Type, Accept and the like. Credentials go in Auth.'
  }
}
