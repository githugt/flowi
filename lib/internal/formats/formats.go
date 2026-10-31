// SPDX-License-Identifier: Apache-2.0

// Package formats holds the names of the OpenAPI Format Registry (https://spec.openapis.org/registry/format/), the
// values the JSON Schema "format" keyword may take in an OpenAPI description. A registry entry is a convention, not
// part of the specification: a tool may support none of it, and a format that is not in the list is legal (it is an
// annotation). A dated copy is kept here so an offline tool can say which formats it recognises; the list changes
// when the registry does.
package formats

import (
	"slices"
	"sort"
)

// RegistryDate is the day the copy below was taken from the registry.
const RegistryDate = "2026-10-29"

// Format is one registry entry.
type Format struct {
	Name string
	// Types are the JSON Schema types the format applies to, as the registry gives them: it says "number" for integers.
	Types      []string
	Meaning    string
	Deprecated bool
}

// entry is a row of the table in registered(): name, types, meaning, deprecated.
type entry struct {
	name       string
	types      []string
	meaning    string
	deprecated bool
}

func registered() []entry {
	str, num, both := []string{"string"}, []string{"number"}, []string{"number", "string"}
	return []entry{
		{"base64url", str, "Binary data encoded as a URL-safe string", true},
		{"binary", str, "Any sequence of octets", true},
		{"byte", str, "Base64-encoded data", true},
		{"char", str, "A single character", false},
		{"commonmark", str, "CommonMark-formatted text", false},
		{"date-time-local", str, "RFC 3339 date-time without timezone", false},
		{"date-time", str, "Date and time per RFC 3339", false},
		{"date", str, "Full-date per RFC 3339", false},
		{"decimal", both, "Fixed-point decimal of unspecified precision and range", false},
		{"decimal128", both, "Decimal floating-point with 34 significant digits", false},
		{"double-int", num, "Integer exactly storable in an IEEE 754 double", false},
		{"double", num, "Double-precision floating-point number", false},
		{"duration", str, "Duration per RFC 3339", false},
		{"email", str, "Email address (Mailbox, RFC 5321)", false},
		{"float", num, "Single-precision floating-point number", false},
		{"hostname", str, "Host name per RFC 1123", false},
		{"html", str, "HTML-formatted text", false},
		{"http-date", str, "Date and time per HTTP-date (RFC 7231)", false},
		{"idn-email", str, "Internationalized email address (RFC 6531)", false},
		{"idn-hostname", str, "Internationalized host name (RFC 5890)", false},
		{"int16", num, "Signed 16-bit integer", false},
		{"int32", num, "Signed 32-bit integer", false},
		{"int64", both, "Signed 64-bit integer", false},
		{"int8", num, "Signed 8-bit integer", false},
		{"ipv4-cidr", str, "IPv4 address in CIDR notation (RFC 4632)", false},
		{"ipv4", str, "IPv4 dotted-quad address", false},
		{"ipv6-cidr", str, "IPv6 address in CIDR-style notation (RFC 4291)", false},
		{"ipv6", str, "IPv6 address", false},
		{"iri-reference", str, "Internationalized resource identifier reference (RFC 3987)", false},
		{"iri", str, "Internationalized resource identifier (RFC 3987)", false},
		{"json-pointer", str, "JSON Pointer string (RFC 6901)", false},
		{"language", str, "Language tag (RFC 5646)", false},
		{"media-range", str, "Media range for Accept-style headers (RFC 9110)", false},
		{"password", str, "Hint that the value should be obscured", false},
		{"regex", str, "Regular expression (ECMA-262)", false},
		{"relative-json-pointer", str, "Relative JSON Pointer string", false},
		{"sf-binary", str, "Structured fields byte sequence (RFC 8941)", false},
		{"sf-boolean", str, "Structured fields boolean (RFC 8941)", false},
		{"sf-decimal", num, "Structured fields decimal (RFC 8941)", false},
		{"sf-integer", num, "Structured fields integer (RFC 8941)", false},
		{"sf-string", str, "Structured fields string (RFC 8941)", false},
		{"sf-token", str, "Structured fields token (RFC 8941)", false},
		{"time-local", str, "RFC 3339 time without timezone", false},
		{"time", str, "Time per RFC 3339 full-time", false},
		{"uint16", num, "Unsigned 16-bit integer", false},
		{"uint32", num, "Unsigned 32-bit integer", false},
		{"uint64", both, "Unsigned 64-bit integer", false},
		{"uint8", num, "Unsigned 8-bit integer", false},
		{"unixtime", both, "Seconds since Jan 1, 1970 (IEEE 1003.1-2024)", false},
		{"uri-reference", str, "URI reference (RFC 3986)", false},
		{"uri-template", str, "URI Template (RFC 6570)", false},
		{"uri", str, "Uniform Resource Identifier (RFC 3986)", false},
		{"uuid", str, "Universally Unique Identifier (RFC 4122)", false},
	}
}

// Lookup returns the registry entry called name. Names are case-sensitive.
func Lookup(name string) (Format, bool) {
	for _, e := range registered() {
		if e.name == name {
			return Format{Name: e.name, Types: slices.Clone(e.types), Meaning: e.meaning, Deprecated: e.deprecated}, true
		}
	}
	return Format{}, false
}

// Names lists every registered format name, sorted.
func Names() []string {
	var out []string
	for _, e := range registered() {
		out = append(out, e.name)
	}
	sort.Strings(out)
	return out
}

// bad is a value that does not fit the format, for the formats where one is plain.
func bad() map[string]string {
	return map[string]string{
		"date-time": "not-a-date-time", "date": "2026-13-45", "time": "25:61:61", "duration": "P-1X", "email": "not-an-email",
		"uuid": "not-a-uuid", "uri": "ht!tp://bad uri", "ipv4": "999.1.1.1", "ipv6": "12345::g", "hostname": "bad host!",
		"http-date": "yesterday-ish", "json-pointer": "no-leading-slash", "regex": "([unclosed",
	}
}

// InvalidSamples lists the formats Invalid has a sample for, sorted.
func InvalidSamples() []string {
	var out []string
	for name := range bad() {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Invalid returns a string that does not fit the registered format name: a value to try a server with. It is false for
// a name with no plain wrong value (most numeric formats are checked by range, not by shape) and for an unregistered one.
func Invalid(name string) (string, bool) {
	sample, ok := bad()[name]
	return sample, ok
}
