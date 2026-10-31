// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// apiKey sends a key in a header, the query string or a cookie.
type apiKey struct{ name, value, where string }

func newAPIKey(p params) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("name", "value"); err != nil {
		return nil, err
	}
	where := strings.ToLower(p.get("in", "header"))
	switch where {
	case "header", "query", "cookie":
	default:
		return nil, fmt.Errorf("apikey auth: in must be header, query or cookie, got %q", where)
	}
	return apiKey{name: p.values["name"], value: p.values["value"], where: where}, nil
}

func (a apiKey) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	switch a.where {
	case "query":
		u, err := url.Parse(r.URL)
		if err != nil {
			return nil, fmt.Errorf("adding the API key to %q: %w", r.URL, err)
		}
		q := u.Query()
		q.Set(a.name, a.value)
		u.RawQuery = q.Encode()
		r.URL = u.String()
	case "cookie":
		r.Header.Add("Cookie", (&cookieValue{a.name, a.value}).String())
	default:
		r.Header.Set(a.name, a.value)
	}
	return []Applied{{Where: a.where, Name: a.name, Value: a.value, Secret: true}}, nil
}

type cookieValue struct{ name, value string }

func (c *cookieValue) String() string { return url.QueryEscape(c.name) + "=" + c.value }
