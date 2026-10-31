// SPDX-License-Identifier: Apache-2.0

// Package importers holds what the importers for other tools (Postman,
// Bruno, HAR, curl) share: the result and per-item report they return,
// the rule that keeps credentials out of imported files, and file-safe
// names. Each importer is a pure transform in a subpackage; the facade
// plans and writes files.
package importers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Result is what a document imports as. Paths are relative to requests/
// and flows/; Environment names become environments/<name>.yaml.
type Result struct {
	Requests     []model.Request
	Environments []model.Environment
	Flows        []model.Flow
	Examples     []Example
	Items        []Item
}

// Item is one thing in the source (a request, a folder, an environment,
// a command) and what happened to it. A skipped item contributes nothing
// to the result; an imported one may still carry warnings.
type Item struct {
	Name    string
	Source  diag.Location
	Skipped bool
	Diags   diag.List
}

// Example is a recorded response kept as a pinned example of a request:
// <request>.examples/<Name>.json, in the format engine.PinExample writes.
type Example struct {
	RequestPath string   `json:"-"`
	Name        string   `json:"-"`
	Version     string   `json:"version,omitempty"`
	Status      int      `json:"status"`
	Headers     []Header `json:"headers,omitempty"`
	Body        any      `json:"body"`
}

// Header is one example header.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Report adds a diagnostic to the item.
func (it *Item) Report(d diag.Diagnostic) {
	if d.File == "" {
		d.Location = it.Source
	}
	it.Diags = append(it.Diags, d)
}

// Unmapped reports something with no Flowi equivalent (FLW-IMPORT-001).
func (it *Item) Unmapped(what, how, detail string) {
	it.UnmappedAt(diag.Location{}, what, how, detail)
}

// UnmappedAt is Unmapped reported at loc.
func (it *Item) UnmappedAt(loc diag.Location, what, how, detail string) {
	it.Report(diag.New(diag.CodeImportUnmapped, loc, map[string]string{"what": what, "how": how, "detail": detail}))
}

// Skip marks the item skipped (FLW-IMPORT-005).
func (it *Item) Skip(detail string) {
	it.Skipped = true
	it.Report(diag.New(diag.CodeImportSkipped, diag.Location{}, map[string]string{"item": it.Name, "detail": detail}))
}

// Redact returns the placeholder that replaces a credential found at
// where (for example "header Authorization") and reports it. The source
// value is never kept: it would end up in a committed file.
func (it *Item) Redact(where, name string) string {
	return it.RedactAt(diag.Location{}, where, name)
}

// RedactAt is Redact reported at loc (the item's source when loc has no
// file).
func (it *Item) RedactAt(loc diag.Location, where, name string) string {
	placeholder := "{{secret." + SecretName(name) + "}}"
	it.Report(diag.New(diag.CodeImportSensitive, loc, map[string]string{"where": where, "placeholder": placeholder}))
	return placeholder
}

var nonSecretName = regexp.MustCompile(`[^a-z0-9_]+`)

// SecretName is the secret variable a credential named name goes into:
// lower case, other characters as "_" (X-Api-Key → x_api_key).
func SecretName(name string) string {
	return strings.Trim(nonSecretName.ReplaceAllString(strings.ToLower(name), "_"), "_")
}

// credentialWords are fragments that make a name a credential, matched
// in the name lower-cased with separators removed (accessToken,
// access_token and X-Access-Token all become "accesstoken").
func credentialWords() []string {
	return []string{"token", "secret", "passw", "pwd", "apikey", "session", "auth", "credential", "jwt", "cookie", "signature"}
}

// notCredentials are names that contain a credential word but are not
// one.
func notCredentials() []string {
	return []string{"author", "authors", "authority", "authored", "authorname"}
}

var nameSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// SensitiveHeader reports names (headers, variables, JSON keys, form
// fields, query parameters) that carry credentials: any name containing
// a credential word, or ending in "key" (privateKey, x-api-key). Erring
// towards redaction is deliberate: a redacted value is visible and fixed
// by hand, a leaked one ends up in git.
func SensitiveHeader(name string) bool {
	compact := nameSeparators.ReplaceAllString(strings.ToLower(name), "")
	if compact == "" || slices.Contains(notCredentials(), compact) {
		return false
	}
	if strings.HasSuffix(compact, "key") {
		return true
	}
	return slices.ContainsFunc(credentialWords(), func(w string) bool { return strings.Contains(compact, w) })
}

// StripUserinfo removes user:password@ from a URL, reporting whether
// there was any. Text that is not an absolute URL is returned as is.
func StripUserinfo(raw string) (string, bool) {
	scheme, rest, ok := strings.Cut(raw, "://")
	if !ok || strings.Contains(scheme, "{{") {
		return raw, false
	}
	authority := rest
	if i := strings.IndexAny(rest, "/?#"); i >= 0 {
		authority = rest[:i]
	}
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return raw, false
	}
	return scheme + "://" + rest[at+1:], true
}

// RedactBodyText returns a body with credential values replaced: JSON
// through RedactJSON, form text (a=1&b=2) pair by pair. Values that are
// variable references stay. Other text is returned as is.
func (it *Item) RedactBodyText(text, where string) string {
	if v, ok := decodeJSON(text); ok {
		clean, n := RedactJSON(v)
		if n == 0 {
			return text
		}
		it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": fmt.Sprintf("%s (%d values)", where, n), "placeholder": Redacted}))
		return encodeJSONLike(clean, text)
	}
	if !formPairs.MatchString(text) {
		return text
	}
	parts := strings.Split(text, "&")
	n := 0
	for i, p := range parts {
		k, v, _ := strings.Cut(p, "=")
		if SensitiveHeader(k) && v != "" && !HasVar(v) {
			parts[i], n = k+"="+Redacted, n+1
		}
	}
	if n > 0 {
		it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": fmt.Sprintf("%s (%d values)", where, n), "placeholder": Redacted}))
	}
	return strings.Join(parts, "&")
}

var formPairs = regexp.MustCompile(`^[^=&\s]+=[^&\s]*(&[^=&\s]+=[^&\s]*)*$`)

func decodeJSON(text string) (any, bool) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "{") && !strings.HasPrefix(t, "[") {
		return nil, false
	}
	var v any
	return v, json.Unmarshal([]byte(t), &v) == nil
}

// encodeJSONLike encodes v compactly, or indented when the original was
// written over several lines.
func encodeJSONLike(v any, original string) string {
	var data []byte
	var err error
	if strings.Contains(strings.TrimSpace(original), "\n") {
		data, err = json.MarshalIndent(v, "", "  ")
	} else {
		data, err = json.Marshal(v)
	}
	if err != nil {
		return Redacted // v came from decoding JSON; this cannot happen
	}
	return string(data)
}

var nonFileName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// maxNameRunes keeps a name within the file-name limit with room for a
// uniqueness suffix.
const maxNameRunes = 56

// FileName makes s a single valid file name (letters, digits, '.', '_',
// '-', not starting with '.'), or "" when nothing is left.
func FileName(s string) string {
	out := strings.TrimLeft(strings.Trim(nonFileName.ReplaceAllString(s, "-"), "-"), ".-")
	if r := []rune(out); len(r) > maxNameRunes {
		out = strings.TrimRight(string(r[:maxNameRunes]), "-.")
	}
	return out
}

// Names hands out unique names: name, name-2, name-3… Names that differ
// only by case count as the same, since they are the same file on macOS
// and Windows. The zero value is ready to use.
type Names struct {
	taken map[string]bool
	next  map[string]int // lower-cased base → next suffix to try
}

// Unique returns path, or the first free path-N, and takes it.
func (n *Names) Unique(path string) string {
	if n.taken == nil {
		n.taken, n.next = map[string]bool{}, map[string]int{}
	}
	out := path
	key := strings.ToLower(path)
	for i := max(n.next[key], 2); n.taken[strings.ToLower(out)]; i++ {
		out = fmt.Sprintf("%s-%d", path, i)
		n.next[key] = i + 1
	}
	n.taken[strings.ToLower(out)] = true
	return out
}

// Diags lists every item's diagnostics, in item order.
func (r Result) Diags() diag.List {
	var out diag.List
	for _, it := range r.Items {
		out = append(out, it.Diags...)
	}
	return out
}

// Skipped counts the skipped items.
func (r Result) Skipped() int {
	n := 0
	for _, it := range r.Items {
		if it.Skipped {
			n++
		}
	}
	return n
}

// Literal makes imported text safe as a Flowi template: each "{{" is
// written as {{ "{{" }}, which renders back to "{{". Other tools' values
// are literal text; without this a "{{" in them would become a template.
func Literal(s string) string {
	return strings.ReplaceAll(s, "{{", `{{ "{{" }}`)
}

// Redacted replaces credential values in recorded data.
const Redacted = "(redacted)"

// RedactJSON returns a copy of v with the value of every key that names a
// credential (the SensitiveHeader rule applied to key names) replaced by
// Redacted, and how many were. Recorded responses keep their shape, which
// is all contract checks need.
func RedactJSON(v any) (any, int) {
	switch x := v.(type) {
	case map[string]any:
		out, n := make(map[string]any, len(x)), 0
		for k, item := range x {
			text, isText := item.(string)
			isReference := isText && HasVar(text) // a {{variable}}, not a credential itself
			if SensitiveHeader(k) && !isReference {
				out[k] = Redacted
				n++
				continue
			}
			var c int
			out[k], c = RedactJSON(item)
			n += c
		}
		return out, n
	case []any:
		out, n := make([]any, len(x)), 0
		for i, item := range x {
			var c int
			out[i], c = RedactJSON(item)
			n += c
		}
		return out, n
	}
	return v, 0
}
