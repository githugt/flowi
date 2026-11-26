// SPDX-License-Identifier: Apache-2.0

package model

// GraphQL is a GraphQL operation sent as the body of an HTTP request: a query, a mutation or a subscription, with
// its variables. The version's URL, Headers, Auth and Method stay what they are for any HTTP request; the method is
// POST (the operation travels as a JSON body) unless it is GET (a query travels in the URL).
type GraphQL struct {
	// Query is the GraphQL document, sent as written. It is not templated: a value belongs in Variables, where it
	// cannot change what the document means (the same rule as a SQL query's params).
	Query string
	// Variables is a JSON object as text. It is templated like a JSON body, so {{env.user_id}} works inside it.
	Variables string
	// Operation names the operation to run when the document holds more than one. Empty: the only one.
	Operation string
	// Receive, for a subscription, says how many events to wait for, for how long, and which count. Nil: the first
	// event, within 30 seconds. It means nothing for a query or a mutation.
	Receive *AsyncReceive
}

// IsZero reports whether nothing is set.
func (g GraphQL) IsZero() bool { return g == GraphQL{} }

// ProtocolGraphQLWS is the transport that carries a subscription (graphql-transport-ws over a WebSocket). It is not
// one of the async protocols a request can name: a subscription is a GraphQL body, and the engine picks this
// transport for it.
const ProtocolGraphQLWS = "graphql-ws"
