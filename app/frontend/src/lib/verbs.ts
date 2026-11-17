const KNOWN_VERBS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'QUERY', 'WS', 'SSE', 'MQTT', 'KAFKA', 'AMQP', 'NATS', 'GRPC', 'SQL', 'REDIS', 'SQS', 'SNS', 'BUS', 'PUBSUB', 'FLOW', 'POSTGRES', 'MYSQL', 'MARIADB', 'SQLITE', 'CSV', 'XLSX', 'PARQUET']

/** The colour name of a verb (GET, MQTT, POSTGRES …): the suffix of its --v-* token. Databases and file stores have their own; others OTHER. */
export function verbKey(verb: string): string {
  const upper = verb.toUpperCase()
  return KNOWN_VERBS.includes(upper) ? upper : 'OTHER'
}

// The verb a message protocol shows where an HTTP request shows its method (the same as model.ProtocolVerb).
export const protocolVerb = (id: string): string => (id === 'servicebus' ? 'BUS' : id === 'amqp10' ? 'AMQP1' : id.toUpperCase())
