// SPDX-License-Identifier: Apache-2.0

package postman

import (
	"encoding/json"
	"fmt"
	"strings"
)

// The Postman v2 collection format, as far as importing reads it.

type collection struct {
	Info *struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	} `json:"info"`
	Item     []item     `json:"item"`
	Auth     *auth      `json:"auth"`
	Event    []event    `json:"event"`
	Variable []variable `json:"variable"`
}

type item struct {
	Name     string          `json:"name"`
	Item     []item          `json:"item"`
	Request  json.RawMessage `json:"request"`
	Response []savedResponse `json:"response"`
	Event    []event         `json:"event"`
	Auth     *auth           `json:"auth"`
}

type variable struct {
	Key      string `json:"key"`
	Value    flex   `json:"value"`
	Type     string `json:"type"`
	Disabled bool   `json:"disabled"`
}

type event struct {
	Listen string `json:"listen"`
	Script struct {
		Exec lines `json:"exec"`
	} `json:"script"`
}

type savedResponse struct {
	Name   string `json:"name"`
	Code   int    `json:"code"`
	Header []kv   `json:"header"`
	Body   string `json:"body"`
}

// kv is a Postman key/value entry (headers, query, form fields, auth
// parameters).
type kv struct {
	Key      string `json:"key"`
	Value    flex   `json:"value"`
	Disabled bool   `json:"disabled"`
	Type     string `json:"type"`
	Src      flex   `json:"src"`
}

// auth is a Postman auth block: its type and that type's parameters.
type auth struct {
	Type   string
	Params []kv
}

func (a *auth) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(raw["type"], &a.Type); err != nil {
		return fmt.Errorf("auth type: %w", err)
	}
	if params, ok := raw[a.Type]; ok && !isJSONString(params) {
		return json.Unmarshal(params, &a.Params)
	}
	return nil
}

func (a *auth) param(key string) string {
	for _, p := range a.Params {
		if p.Key == key {
			return string(p.Value)
		}
	}
	return ""
}

// flex is a value Postman writes as a string, number, boolean or null.
type flex string

func (f *flex) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case nil:
		*f = ""
	case string:
		*f = flex(x)
	default:
		*f = flex(strings.TrimSpace(string(data)))
	}
	return nil
}

// lines is a script's exec: an array of lines or one string.
type lines []string

func (l *lines) UnmarshalJSON(data []byte) error {
	if isJSONString(data) {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*l = []string{s}
		return nil
	}
	return json.Unmarshal(data, (*[]string)(l))
}

type body struct {
	Mode    string `json:"mode"`
	Raw     string `json:"raw"`
	Options *struct {
		Raw *struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
	URLEncoded []kv `json:"urlencoded"`
	FormData   []kv `json:"formdata"`
	File       *struct {
		Src flex `json:"src"`
	} `json:"file"`
	GraphQL *struct {
		Query     string `json:"query"`
		Variables string `json:"variables"`
	} `json:"graphql"`
}

// decoded is a request item in one shape: Postman also allows the request
// to be just its URL, and the URL to be a string or an object.
type decoded struct {
	method  string
	headers []kv
	body    *body
	url     urlParts
	auth    *auth
}

type urlParts struct {
	raw      string
	query    []kv
	hasQuery bool
	vars     []kv
}

func decodeRequest(raw json.RawMessage) (decoded, error) {
	if isJSONString(raw) {
		var u string
		err := json.Unmarshal(raw, &u)
		return decoded{url: urlParts{raw: u}}, err
	}
	var r struct {
		Method string          `json:"method"`
		Header []kv            `json:"header"`
		Body   *body           `json:"body"`
		URL    json.RawMessage `json:"url"`
		Auth   *auth           `json:"auth"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return decoded{}, err
	}
	u, err := decodeURL(r.URL)
	return decoded{method: r.Method, headers: r.Header, body: r.Body, url: u, auth: r.Auth}, err
}

func decodeURL(raw json.RawMessage) (urlParts, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return urlParts{}, nil
	}
	if isJSONString(raw) {
		var s string
		err := json.Unmarshal(raw, &s)
		return urlParts{raw: s}, err
	}
	var u struct {
		Raw      string `json:"raw"`
		Query    []kv   `json:"query"`
		Variable []kv   `json:"variable"`
	}
	err := json.Unmarshal(raw, &u)
	return urlParts{raw: u.Raw, query: u.Query, hasQuery: u.Query != nil, vars: u.Variable}, err
}
