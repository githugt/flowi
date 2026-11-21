import type { GRPCBlock } from './types'

// sourceKey names where a gRPC request's method list comes from: the server it asks, or the .proto files it reads.
// The list is stale when the key has changed since the list was loaded.
// Before: the list was loaded once. A fixed address or an edited .proto file kept the old list, or the old error,
// until the user closed and reopened the request.
export function sourceKey(b: Pick<GRPCBlock, 'Server' | 'Proto' | 'Import'>): string {
  return JSON.stringify([b.Server, b.Proto ?? [], b.Import ?? []])
}
