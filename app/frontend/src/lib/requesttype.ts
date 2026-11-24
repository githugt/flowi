// A type of request the bar under the address offers: HTTP, gRPC, messaging, database or data file. Which protocol,
// database or file format a request uses is the address row's own pill, not part of the type.

export interface TypeChoice {
  id: string
  label: string
  icon: string
}
