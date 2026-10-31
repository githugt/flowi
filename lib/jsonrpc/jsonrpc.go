// SPDX-License-Identifier: Apache-2.0

// Package jsonrpc writes and reads the JSON-RPC 2.0 request envelope. A
// JSON-RPC request in Flowi is an ordinary HTTP request whose JSON body is
// this envelope, so it carries headers, auth, hooks and checks like any
// other; this package is what an editor and the OpenRPC importer share.
package jsonrpc

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// Version is the protocol version the envelope states.
const Version = "2.0"

// Call is a JSON-RPC request. Params is the JSON text of the parameters (an
// array or an object; it may hold {{templates}}), empty for none. ID is a
// number or a text; empty makes the call a notification, which has no id and
// gets no answer.
type Call struct {
	Method string
	Params string
	ID     string
}

// Envelope writes c as the JSON body to send, indented. An ID of digits is
// written as a number, any other as a string. Params that do not parse yet
// (a {{template}} outside quotes) are written as typed.
func Envelope(c Call) string {
	var b strings.Builder
	b.WriteString("{\n  \"jsonrpc\": \"" + Version + "\",\n  \"method\": " + quote(c.Method))
	if params := strings.TrimSpace(c.Params); params != "" {
		b.WriteString(",\n  \"params\": " + indented(params))
	}
	if c.ID != "" {
		b.WriteString(",\n  \"id\": " + idJSON(c.ID))
	}
	b.WriteString("\n}")
	return b.String()
}

// Parse reads a JSON-RPC 2.0 request envelope. ok is false for anything
// else: text that is not JSON, not an object, or without "jsonrpc": "2.0" and
// a method name.
func Parse(text string) (Call, bool) {
	var raw struct {
		JSONRPC string          `json:"jsonrpc"`
		Method  *string         `json:"method"`
		Params  json.RawMessage `json:"params"`
		ID      json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil || raw.JSONRPC != Version || raw.Method == nil || *raw.Method == "" {
		return Call{}, false
	}
	id, ok := idText(raw.ID)
	if !ok {
		return Call{}, false
	}
	c := Call{Method: *raw.Method, ID: id}
	if len(raw.Params) > 0 {
		var out bytes.Buffer
		if json.Indent(&out, raw.Params, "", "  ") == nil {
			c.Params = out.String()
		}
	}
	return c, true
}

func quote(s string) string {
	data, _ := json.Marshal(s) // a string always marshals
	return string(data)
}

// indented re-indents valid JSON to sit under a two-space key; other text is left as typed.
func indented(params string) string {
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(params), "  ", "  "); err != nil {
		return params
	}
	return out.String()
}

func idJSON(id string) string {
	if _, err := strconv.ParseInt(id, 10, 64); err == nil {
		return id
	}
	return quote(id)
}

// idText reads an id: a number, a string, or null/absent (a notification).
func idText(raw json.RawMessage) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s, true
	}
	var n json.Number
	if json.Unmarshal(raw, &n) == nil {
		return n.String(), true
	}
	return "", false
}
