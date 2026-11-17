const KNOWN_VERBS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'QUERY', 'WS', 'SSE', 'MQTT', 'KAFKA', 'AMQP', 'NATS', 'GRPC', 'SQL', 'REDIS', 'SQS', 'SNS', 'BUS', 'PUBSUB', 'FLOW', 'POSTGRES', 'MYSQL', 'MARIADB', 'SQLITE']
// File stores have no colour of their own: they take SQL's.
const FILE_STORES = ['CSV', 'XLSX', 'PARQUET']

/** The colour name of a verb (GET, MQTT, POSTGRES …): the suffix of its --v-* token. The databases have their own; file stores take SQL's; others OTHER. */
export function verbKey(verb: string): string {
  const upper = verb.toUpperCase()
  return FILE_STORES.includes(upper) ? 'SQL' : KNOWN_VERBS.includes(upper) ? upper : 'OTHER'
}

// The verb a message protocol shows where an HTTP request shows its method (the same as model.ProtocolVerb).
export const protocolVerb = (id: string): string => (id === 'servicebus' ? 'BUS' : id === 'amqp10' ? 'AMQP1' : id.toUpperCase())
