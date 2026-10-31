// SPDX-License-Identifier: Apache-2.0

package importers

import (
	"regexp"
	"strconv"
	"strings"
)

// dynamicVariables maps Postman and Bruno's dynamic variables to Flowi generators.
func dynamicVariables() map[string]string {
	return map[string]string{
		"$guid": "$uuid-v4", "$randomUUID": "$uuid-v4",
		"$timestamp": "$timestamp", "$isoTimestamp": "$now",
		"$randomInt":       "$int(0, 1000)",
		"$randomFirstName": "$first-name", "$randomLastName": "$last-name", "$randomFullName": "$full-name",
		"$randomEmail": "$email", "$randomExampleEmail": "$email",
		"$randomCity": "$city-name", "$randomPhoneNumber": "$phone",
		"$randomWord": "$word", "$randomBoolean": "$bool",
	}
}

var varIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// VarRef is the Flowi reference for a Postman or Bruno variable: secret.x for
// credential-like names (the same rule that declares them secret when
// environments import), env.x otherwise.
func VarRef(name string) string {
	ns := "env"
	if SensitiveHeader(name) {
		ns = "secret"
	}
	if varIdentifier.MatchString(name) {
		return ns + "." + name
	}
	return ns + "[" + strconv.Quote(name) + "]"
}

// Translate rewrites Postman's and Bruno's {{var}} references as Flowi
// templates and returns the dynamic variables it has no generator for.
// It is Translator.Text without runtime variables or an item.
func Translate(s string) (string, []string) {
	var t Translator
	out := t.Text(s)
	return out, t.unknown
}

// VarExpression is the Flowi expression for a {{name}}: a generator for
// dynamic variables it knows, else a variable reference; unknown dynamic
// variables are appended to unknown.
func VarExpression(name string, unknown *[]string) string {
	if env, ok := strings.CutPrefix(name, "process.env."); ok {
		return VarRef(env) // Bruno reads the shell environment; Flowi's environment plays that part
	}
	if !strings.HasPrefix(name, "$") {
		return VarRef(name)
	}
	if gen, ok := dynamicVariables()[name]; ok {
		return gen
	}
	*unknown = append(*unknown, name)
	return VarRef(strings.TrimPrefix(name, "$"))
}

// SingleVar returns the variable when s is exactly one {{var}}.
func SingleVar(s string) (string, bool) {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "{{") || !strings.HasSuffix(t, "}}") || strings.Count(t, "{{") != 1 {
		return "", false
	}
	return strings.TrimSpace(t[2 : len(t)-2]), true
}

// HasVar reports whether s holds a {{…}} reference.
func HasVar(s string) bool { return strings.Contains(s, "{{") && strings.Contains(s, "}}") }
