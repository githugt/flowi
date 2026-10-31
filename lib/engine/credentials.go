// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"strings"
)

// CredentialHeaders names the headers of rec's response that look like
// credentials: cookies being set, tokens, keys, sessions. The masker only
// knows secrets Flowi holds, so a session cookie a server hands out passes
// through it; this is what lets the window warn before such a response is
// pinned, since a pinned example is committed with the collection. Names are
// as the server sent them, once each, in order.
func CredentialHeaders(rec Record) []string {
	var out []string
	for _, h := range rec.Header {
		if looksLikeCredential(h.Name) && !slices.ContainsFunc(out, func(n string) bool { return strings.EqualFold(n, h.Name) }) {
			out = append(out, h.Name)
		}
	}
	return out
}

func looksLikeCredential(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "set-cookie", "set-cookie2", "authorization", "proxy-authorization", "cookie":
		return true
	}
	for _, word := range []string{"token", "secret", "api-key", "apikey", "password", "session", "auth-key", "csrf", "xsrf"} {
		if strings.Contains(n, word) {
			return true
		}
	}
	return false
}
