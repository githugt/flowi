// SPDX-License-Identifier: Apache-2.0

package importers

import (
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Translator turns another tool's {{var}} text into Flowi templates for
// one item, and applies the credential rules there. Runtime names are
// variables the source sets while running (Bruno's post-response vars,
// which import as extracts): they are read bare ({{token}}), which looks
// in the flow's vars first and the environment after.
type Translator struct {
	Item    *Item
	Runtime map[string]bool
	unknown []string
}

// Text translates every {{var}}; an unclosed "{{" stays literal.
func (t *Translator) Text(s string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			b.WriteString(s[:start])
			b.WriteString(Literal(s[start:]))
			return b.String()
		}
		b.WriteString(s[:start])
		b.WriteString("{{" + t.Expression(strings.TrimSpace(s[start+2:start+2+end])) + "}}")
		s = s[start+2+end+2:]
	}
}

// Expression is the Flowi expression for {{name}}.
func (t *Translator) Expression(name string) string {
	if t.Runtime[name] && varIdentifier.MatchString(name) {
		return name
	}
	return VarExpression(name, &t.unknown)
}

// ReportUnknown reports each dynamic variable Text met without a
// generator (they became env references, so validate flags them).
func (t *Translator) ReportUnknown() {
	slices.Sort(t.unknown)
	for _, name := range slices.Compact(t.unknown) {
		t.Item.Unmapped("dynamic variable "+name, "replaced by an env reference", "Flowi has no generator for it")
	}
}

// Credential translates a value, unless it is a credential written out
// literally: that is replaced by a secret placeholder. A reference to a
// variable ({{token}}) is not a credential itself.
func (t *Translator) Credential(where, name, value string) string {
	if SensitiveHeader(name) && value != "" && !HasVar(value) {
		return t.Item.Redact(where, name)
	}
	return t.Text(value)
}

// Bearer is an Authorization header value for a bearer token.
func (t *Translator) Bearer(token string) string {
	if !HasVar(token) {
		return t.Item.Redact("bearer token", "Authorization")
	}
	return "Bearer " + t.Text(token)
}

// Basic is Basic {{toBase64(user + ":" + password)}}, with variables as
// expressions and a literal password as secret.password. Text mixed with
// variables has no single expression: the header is redacted instead.
func (t *Translator) Basic(user, password string) string {
	u, okUser := t.basicPart(user, false)
	p, okPass := t.basicPart(password, true)
	if !okUser || !okPass {
		return t.Item.Redact("basic auth", "Authorization")
	}
	return `Basic {{toBase64(` + u + ` + ":" + ` + p + `)}}`
}

func (t *Translator) basicPart(s string, password bool) (string, bool) {
	if name, ok := SingleVar(s); ok {
		return t.Expression(name), true
	}
	switch {
	case HasVar(s):
		return "", false
	case password && s != "":
		t.Item.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": "basic auth password", "placeholder": "{{secret.password}}"}))
		return "secret.password", true
	}
	return strconv.Quote(s), true
}

// APIKey is the value of an API key; it is always treated as a credential.
func (t *Translator) APIKey(key, value string) string {
	if !HasVar(value) {
		return t.Item.Redact("API key "+key, key)
	}
	return t.Text(value)
}

// AuthBlock builds the auth block of a scheme from name, value pairs (the
// model's parameter names). A credential written out literally becomes a
// secret placeholder, as everywhere else; a {{variable}} stays a reference.
// Empty values are left out.
func (t *Translator) AuthBlock(scheme string, pairs ...string) *model.Auth {
	secret := model.AuthSecretParams(scheme)
	a := &model.Auth{Type: scheme}
	for i := 0; i+1 < len(pairs); i += 2 {
		name, value := pairs[i], pairs[i+1]
		if value == "" {
			continue
		}
		if slices.Contains(secret, name) && !HasVar(value) {
			value = t.Item.Redact(scheme+" auth "+name, name)
		} else {
			value = t.Text(value)
		}
		a.Params = append(a.Params, model.KeyValue{Key: name, Value: value})
	}
	return a
}
