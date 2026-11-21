// SPDX-License-Identifier: Apache-2.0

// Package credname decides whether a name (a header, a variable, a JSON key, a
// form field, a query parameter) carries a credential. Redaction on import and
// header dropping on a cross-origin redirect both ask it, so one list serves
// both and a word added here protects both.
//
// Before: the importers and the redirect code each kept their own list. The
// redirect list lacked pwd, apikey and jwt, so "X-Jwt" followed a redirect to
// another origin; the importer list was the only one that knew "author" is not
// a credential.
package credname

import (
	"regexp"
	"slices"
	"strings"
)

// words are fragments that make a name a credential, matched in the name
// lower-cased with separators removed (accessToken, access_token and
// X-Access-Token all become "accesstoken").
func words() []string {
	return []string{"token", "secret", "passw", "pwd", "apikey", "session", "auth", "credential", "jwt", "cookie", "signature"}
}

// notCredentials are names that contain a credential word but are not one.
func notCredentials() []string {
	return []string{"author", "authors", "authority", "authored", "authorname"}
}

var separators = regexp.MustCompile(`[^a-z0-9]+`)

// Sensitive reports whether name carries a credential: it contains a
// credential word, or ends in "key" (privateKey, x-api-key). Erring towards
// redaction is deliberate: a redacted value is visible and fixed by hand, a
// leaked one ends up in git.
func Sensitive(name string) bool { return match(name, false) }

// SensitiveAnywhere is Sensitive with "key" counted anywhere in the name, not
// only at its end. A header that merely contains "key" (Idempotency-Key-Id) is
// then also taken for a credential, the safe side to err on when deciding what
// leaves for another origin.
func SensitiveAnywhere(name string) bool { return match(name, true) }

func match(name string, keyAnywhere bool) bool {
	compact := separators.ReplaceAllString(strings.ToLower(name), "")
	if compact == "" || slices.Contains(notCredentials(), compact) {
		return false
	}
	if strings.HasSuffix(compact, "key") || (keyAnywhere && strings.Contains(compact, "key")) {
		return true
	}
	return slices.ContainsFunc(words(), func(w string) bool { return strings.Contains(compact, w) })
}
