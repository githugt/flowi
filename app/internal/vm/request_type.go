// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// The ids of the request types: the six kinds. Which protocol, database or file format a message, database or
// data file request uses is not part of the type; the address row's own picker chooses it.
const (
	TypeHTTP    = "http"
	TypeGraphQL = "graphql"
	TypeGRPC    = "grpc"
	TypeMessage = "message"
	TypeDB      = "db"
	TypeFile    = "file"
)

// TypeFlow is the type of a flow in the sidebar's filters and labels. It is not one of TypeIDs: a request cannot be
// switched to it.
const TypeFlow = "flow"

// TypeName is the name a person reads for a type id: "HTTP", "gRPC", "Async" for the message family, "Database", "Data
// file" and "Flow". An id it does not know is returned as it is.
func TypeName(id string) string {
	switch id {
	case TypeHTTP:
		return "HTTP"
	case TypeGraphQL:
		return "GraphQL"
	case TypeGRPC:
		return "gRPC"
	case TypeMessage:
		return "Async"
	case TypeDB:
		return "Database"
	case TypeFile:
		return "Data file"
	case TypeFlow:
		return "Flow"
	}
	return id
}

// TypeOf is the id of the type of request a version is.
func TypeOf(v model.RequestVersion) string {
	switch {
	case v.GRPC != nil:
		return TypeGRPC
	case v.Async != nil:
		return TypeMessage
	case v.File != nil:
		return TypeFile
	case v.Dataset != nil:
		return TypeDB
	case v.Body.GraphQL != nil:
		return TypeGraphQL
	}
	return TypeHTTP
}

// TypeIDs are every type a request can be switched to, in the order a menu lists them.
func TypeIDs() []string {
	return []string{TypeHTTP, TypeGraphQL, TypeGRPC, TypeMessage, TypeDB, TypeFile}
}

// TypeIcon is the icon a type is drawn with.
func TypeIcon(id string) string {
	switch id {
	case TypeHTTP:
		return "globe"
	case TypeGraphQL:
		return "graphql"
	case TypeGRPC:
		return "grpc"
	case TypeMessage:
		return "message"
	case TypeDB:
		return "database"
	}
	return "table"
}

// ErrUnknownType is a type id that is not one of TypeIDs.
var ErrUnknownType = errors.New("that is not a request type")

// SwitchType changes the selected version to another type of request and returns what it had to leave behind.
// What the types share stays (the checks, values taken from the response, hooks, documentation, headers where the
// new type has them) and so does what can be carried across (an address, a message). It is an ordinary edit: it can
// be discarded, which brings back all of it.
//
// Before: a request could not change type, so a request made as HTTP and then wanted as a WebSocket or a gRPC call
// was made again from nothing.
func (r *Request) SwitchType(id string) (lost []string, err error) {
	if len(r.draft.Versions) == 0 || r.deleting {
		return nil, nil
	}
	if id == TypeOf(r.Current()) {
		return nil, nil
	}
	next, lost, err := ConvertVersion(r.Current(), id)
	if err != nil {
		return nil, err
	}
	r.Edit(func(v *model.RequestVersion) { *v = next })
	r.rawRows, r.problems = nil, nil
	r.chosenKind, r.chosenFormat, r.chosenMultipart = "", "", false
	r.changed()
	return lost, nil
}

// SwitchNote says in a sentence that the request is now of the type called label, and what was left behind.
func SwitchNote(label string, lost []string) string {
	if len(lost) == 0 {
		return "Switched to " + label + ". Discard goes back."
	}
	return "Switched to " + label + ". Left behind: " + strings.Join(lost, ", ") + ". Discard brings them back."
}

// carry is what a version of one type hands to another: its address, message, headers and the like.
type carry struct {
	address string // a URL or a server, as written
	body    model.Body
	headers []model.KeyValue
	query   []model.KeyValue
	method  string // an HTTP method other than GET
	lost    []string
}

// ConvertVersion is v as a request of the type id, and what it could not keep, in words ("the query parameters").
// A message request starts as a WebSocket, a database one as PostgreSQL and a data file one as CSV; the pill in the
// address row then changes that.
func ConvertVersion(v model.RequestVersion, id string) (model.RequestVersion, []string, error) {
	if !slices.Contains(TypeIDs(), id) {
		return v, nil, ErrUnknownType
	}
	from := TypeOf(v)
	if from == id {
		return v, nil, nil
	}
	c := carryOf(v)
	out := v
	out.Method, out.URL, out.Query, out.Body = "", "", nil, model.Body{}
	out.Headers = nil
	out.Async, out.GRPC, out.Dataset, out.File = nil, nil, nil, nil
	out.Variables = slices.Clone(v.Variables)
	httpLike := id == TypeHTTP || id == TypeGraphQL // both send an HTTP request: the status they expect and their auth stay

	if !httpLike && v.Expect != "" {
		c.lost = append(c.lost, "the expected status")
		out.Expect = ""
	}
	if !httpLike && v.Auth != nil {
		c.lost = append(c.lost, "the authentication")
		out.Auth = nil
	}

	switch id {
	case TypeHTTP:
		toHTTP(&out, &c)
	case TypeGraphQL:
		toGraphQL(&out, &c)
	case TypeGRPC:
		toGRPC(&out, &c)
	case TypeMessage:
		if err := toAsync(&out, &c, model.ProtocolWS); err != nil {
			return v, nil, err
		}
	case TypeDB:
		d, err := flowi.StarterDataset(model.DriverPostgres)
		if err != nil {
			return v, nil, err
		}
		out.Dataset = &d
		c.leaveAll()
	default:
		f, err := flowi.StarterFile(model.FormatCSV)
		if err != nil {
			return v, nil, err
		}
		out.File = &f
		c.leaveAll()
	}
	return out, dedupe(c.lost), nil
}

// carryOf reads what v has that another type could take, and notes what only its own type uses.
func carryOf(v model.RequestVersion) carry {
	var c carry
	switch {
	case v.GRPC != nil:
		g := v.GRPC
		c.address, c.body, c.headers = g.Server, g.Body, v.Headers
		if g.Method != "" {
			c.lost = append(c.lost, "the method "+g.Method)
		}
		if len(g.Proto) > 0 || len(g.Import) > 0 {
			c.lost = append(c.lost, "the proto files")
		}
	case v.Async != nil:
		a := v.Async
		if AsyncHeadersAreConnection(a.Protocol) {
			c.address, c.headers = model.TargetURL(a.Protocol, a.Server, a.Channel), v.Headers
		} else if a.Server != "" || a.Channel != "" {
			c.lost = append(c.lost, "the server and "+strings.ToLower(model.AsyncEndpointHint(a.Protocol).Channel))
		}
		if a.Send != nil {
			c.body = a.Send.Body
			if !AsyncHeadersAreConnection(a.Protocol) {
				c.headers = a.Send.Headers
			}
		}
		if len(a.Binding) > 0 {
			c.lost = append(c.lost, "the protocol's options")
		}
		if a.Receive != nil && (a.Receive.Count > 0 || a.Receive.Timeout > 0 || a.Receive.Match != "") {
			c.lost = append(c.lost, "what it waits for")
		}
	case v.SQL() != nil:
		d := v.SQL()
		if strings.TrimSpace(d.Query) != "" {
			c.lost = append(c.lost, "the query")
		}
		if d.Server != "" {
			c.lost = append(c.lost, "where the data is")
		}
		if len(d.Params) > 0 {
			c.lost = append(c.lost, "the query's values")
		}
	default:
		c.address, c.body, c.headers, c.query = v.URL, v.Body, v.Headers, v.Query
		if g := v.Body.GraphQL; g != nil {
			// Another type has no GraphQL body: the operation travels as the JSON it is sent as.
			c.body = model.Body{}
			if !g.IsZero() {
				c.body.JSON = graphQLPayloadJSON(*g)
			}
		}
		// GET and POST are what any type implies (a message is sent, a body is posted), so only another method is noted.
		if m := strings.ToUpper(v.Method); m != "" && m != "GET" && m != "POST" {
			c.method = m
		}
	}
	return c
}

// leaveAll is for a type that takes none of the request's address, message or headers.
func (c *carry) leaveAll() {
	if c.address != "" {
		c.lost = append(c.lost, "the address")
	}
	if bodyHasContent(c.body) {
		c.lost = append(c.lost, "the body")
	}
	if len(c.headers) > 0 {
		c.lost = append(c.lost, "the headers")
	}
	c.leaveQuery()
}

func (c *carry) leaveQuery() {
	if len(c.query) > 0 {
		c.lost = append(c.lost, "the query parameters")
	}
	if c.method != "" {
		c.lost = append(c.lost, "the method "+c.method)
	}
}

// jsonOrText is body when it is a message a non-HTTP type can send, else it says so in lost.
func (c *carry) jsonOrText() model.Body {
	b := c.body
	if len(b.Form) > 0 || len(b.Multipart) > 0 || b.File != "" {
		c.lost = append(c.lost, "the body (a form or a file)")
		return model.Body{}
	}
	return b
}

func toHTTP(out *model.RequestVersion, c *carry) {
	out.Method = "GET"
	if bodyHasContent(c.body) {
		out.Method = "POST"
	}
	if c.method != "" {
		out.Method = c.method
	}
	out.URL = httpAddress(c.address)
	out.Body, out.Headers, out.Query = c.body, c.headers, c.query
}

// toGraphQL makes a GraphQL operation of what an HTTP request had: its address, headers and query parameters stay.
// A JSON body that is already {"query": …} becomes the operation; any other body cannot be one and is left behind.
func toGraphQL(out *model.RequestVersion, c *carry) {
	out.Method = "POST"
	out.URL = httpAddress(c.address)
	out.Headers, out.Query = c.headers, c.query
	g := model.GraphQL{}
	if from, ok := graphQLOfJSON(c.body.JSON); ok {
		g = from
	} else if bodyHasContent(c.body) {
		c.lost = append(c.lost, "the body")
	}
	out.Body = model.Body{GraphQL: &g}
	if c.method != "" {
		c.lost = append(c.lost, "the method "+c.method)
	}
}

// graphQLOfJSON reads a GraphQL-over-HTTP JSON body ({"query", "variables", "operationName"}); false for any other.
func graphQLOfJSON(text string) (model.GraphQL, bool) {
	var doc struct {
		Query         string          `json:"query"`
		Variables     json.RawMessage `json:"variables"`
		OperationName string          `json:"operationName"`
	}
	if json.Unmarshal([]byte(text), &doc) != nil || strings.TrimSpace(doc.Query) == "" {
		return model.GraphQL{}, false
	}
	g := model.GraphQL{Query: doc.Query, Operation: doc.OperationName}
	if len(doc.Variables) > 0 && string(doc.Variables) != "null" {
		g.Variables = string(doc.Variables)
	}
	return g, true
}

func toGRPC(out *model.RequestVersion, c *carry) {
	server, path := splitAddress(c.address)
	if path != "" && path != "/" {
		c.lost = append(c.lost, "the path "+path)
	}
	g := model.GRPC{Server: swapScheme(server, map[string]string{"https": model.GRPCSchemeTLS, "http": model.GRPCSchemePlaintext, "wss": model.GRPCSchemeTLS, "ws": model.GRPCSchemePlaintext}), Body: c.jsonOrText()}
	out.GRPC, out.Headers = &g, c.headers
	c.leaveQuery()
}

func toAsync(out *model.RequestVersion, c *carry, protocol string) error {
	a, err := flowi.StarterAsync(protocol)
	if err != nil {
		return err
	}
	connection := AsyncHeadersAreConnection(protocol)
	switch protocol {
	case model.ProtocolWS:
		server, path := splitAddress(c.address)
		a.Server, a.Channel = swapScheme(server, map[string]string{"https": "wss", "http": "ws", "grpcs": "wss", "grpc": "ws"}), path
	case model.ProtocolSSE:
		a.Server = swapScheme(c.address, map[string]string{"grpcs": "https", "grpc": "http", "wss": "https", "ws": "http"})
	default:
		if c.address != "" {
			c.lost = append(c.lost, "the address")
		}
	}
	if a.Send != nil {
		a.Send.Body = c.jsonOrText()
	} else if bodyHasContent(c.body) {
		c.lost = append(c.lost, "the body")
	}
	switch {
	case connection:
		out.Headers = c.headers
	case a.Send != nil:
		a.Send.Headers = c.headers
	case len(c.headers) > 0:
		c.lost = append(c.lost, "the headers")
	}
	c.leaveQuery()
	out.Async = a
	return nil
}

// splitAddress cuts an address into the server (scheme, host and port) and the path after it, ignoring a "/" inside
// a {{template}}: "{{env.base_url}}/users" is the server "{{env.base_url}}" and the path "/users".
func splitAddress(addr string) (server, path string) {
	addr = strings.TrimSpace(addr)
	start := 0
	if i := strings.Index(addr, "://"); i >= 0 {
		start = i + 3
	}
	depth := 0
	for i := start; i < len(addr); i++ {
		switch {
		case strings.HasPrefix(addr[i:], "{{"):
			depth++
			i++
		case strings.HasPrefix(addr[i:], "}}") && depth > 0:
			depth--
			i++
		case addr[i] == '/' && depth == 0:
			return addr[:i], addr[i:]
		}
	}
	return addr, ""
}

// swapScheme writes the scheme of addr as the table says (https to wss), leaving one that is not in it.
func swapScheme(addr string, table map[string]string) string {
	scheme, rest, ok := strings.Cut(addr, "://")
	if !ok {
		return addr
	}
	if to, found := table[strings.ToLower(scheme)]; found {
		return to + "://" + rest
	}
	return addr
}

// httpAddress is an address of any type as an HTTP one: wss and grpcs become https.
func httpAddress(addr string) string {
	return swapScheme(addr, map[string]string{"wss": "https", "ws": "http", "grpcs": "https", "grpc": "http"})
}

func dedupe(list []string) []string {
	var out []string
	for _, s := range list {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}
