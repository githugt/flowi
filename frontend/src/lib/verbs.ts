// The verb a message protocol shows where an HTTP request shows its method (the same as model.ProtocolVerb).
export const protocolVerb = (id: string): string => (id === 'servicebus' ? 'BUS' : id.toUpperCase())
