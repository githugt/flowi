// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"net/url"
	"slices"
	"strings"
)

// hidden replaces each part of a connection string an error quoted.
const hidden = "***"

// minHiddenLength is the shortest part worth hiding: a shorter one ("db", a one-digit port) is a common piece of
// ordinary words, and replacing it would mangle the message and hide nothing.
const minHiddenLength = 3

// redactedError is an error whose text no longer holds what the connection string held. It still unwraps to the
// original, so errors.Is and errors.As work (the text is what a person or a log would see).
type redactedError struct {
	text  string
	cause error
}

func (e *redactedError) Error() string { return e.text }
func (e *redactedError) Unwrap() error { return e.cause }

// redactServer hides the login, the address and the database of server wherever err quotes them. Drivers echo them in
// their errors ("failed to connect to user=… database=…", "Access denied for user …@host"), and the engine's masking
// only knows whole secret values, not the pieces of a connection string kept as one secret.
func redactServer(err error, server string) error {
	text := err.Error()
	out := text
	for _, part := range sensitiveParts(server) {
		out = strings.ReplaceAll(out, part, hidden)
	}
	if out == text {
		return err
	}
	return &redactedError{text: out, cause: err}
}

// sensitiveParts are the parts of server to hide, longest first so a part that holds another is replaced whole.
func sensitiveParts(server string) []string {
	parts := []string{server}
	parts = append(parts, urlParts(server)...)
	parts = append(parts, keyValueParts(server)...)
	parts = slices.DeleteFunc(parts, func(p string) bool { return len(p) < minHiddenLength })
	slices.SortStableFunc(parts, func(a, b string) int { return len(b) - len(a) })
	return slices.Compact(parts)
}

// urlParts are the user, password, host, port and database of a URL; none for a text that is not one.
func urlParts(server string) []string {
	u, err := url.Parse(server)
	if err != nil || u.Host == "" {
		return nil
	}
	password, _ := u.User.Password()
	return []string{u.User.Username(), password, u.Hostname(), u.Port(), strings.TrimPrefix(u.Path, "/")}
}

// keyValueParts are the values of a "host=… user=… password=…" string (PostgreSQL accepts one); none for a text that is not one.
func keyValueParts(server string) []string {
	var out []string
	for _, field := range strings.Fields(server) {
		if _, value, found := strings.Cut(field, "="); found {
			out = append(out, value)
		}
	}
	return out
}
