// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"strings"
)

// StandardMethods are the HTTP methods of the core standards, in the order a picker offers them. QUERY is RFC 10008
// (June 2026): a safe, idempotent request that carries a body.
func StandardMethods() []string {
	return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "QUERY"}
}

// MoreMethods are other methods in use that a person may need: the WebDAV and CalDAV family and SEARCH (registered with
// IANA), LINK and UNLINK, and PURGE, which CDNs and caches use and which is not registered.
func MoreMethods() []string {
	return []string{
		"PROPFIND", "PROPPATCH", "MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK", "REPORT", "SEARCH", "MKCALENDAR",
		"LINK", "UNLINK", "PURGE",
	}
}

// KnownMethod says whether name is a method Flowi knows by name, ignoring case, and returns its usual spelling.
// Method names are case-sensitive on the wire: "query" is not QUERY, so a lower-case known name is worth a warning.
func KnownMethod(name string) (spelling string, known bool) {
	upper := strings.ToUpper(name)
	if slices.Contains(StandardMethods(), upper) || slices.Contains(MoreMethods(), upper) {
		return upper, true
	}
	return "", false
}

// ValidMethod says whether name can be sent as an HTTP method: a token (RFC 9110), which is one or more of
// letters, digits and !#$%&'*+-.^_`|~ and nothing else. It does not say the server knows it.
func ValidMethod(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !isTokenRune(r) {
			return false
		}
	}
	return true
}

func isTokenRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", r)
}
