// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"os"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

const envPrefix = "FLOWI_"

// reservedEnv are FLOWI_ variables that are not settings.
func reservedEnv(name string) bool {
	return name == "FLOWI_CONFIG_DIR" || name == "FLOWI_VAULT_KEY" || strings.HasPrefix(name, "FLOWI_SECRET_")
}

// EnvVarName returns the environment variable that overrides a key.
func EnvVarName(key string) string {
	return envPrefix + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(key))
}

// FromEnviron reads FLOWI_* overrides from an environ slice ("K=V").
// Unknown FLOWI_ variables warn, so typos do not pass silently.
func FromEnviron(schema []Key, environ []string) (LayerValues, diag.List) {
	byVar := make(map[string]Key, len(schema))
	for _, k := range schema {
		byVar[EnvVarName(k.Name)] = k
	}
	lv := LayerValues{Layer: LayerEnv, Source: "environment", Values: model.Settings{}}
	var diags diag.List
	for _, entry := range environ {
		name, value, _ := strings.Cut(entry, "=")
		// An empty value counts as unset, as shells and CI systems commonly
		// "clear" a variable by assigning nothing.
		if !strings.HasPrefix(name, envPrefix) || reservedEnv(name) || value == "" {
			continue
		}
		loc := diag.Location{File: name}
		k, ok := byVar[name]
		if !ok {
			diags = append(diags, diag.New(diag.CodeSettingUnknown, loc, map[string]string{"key": name}))
			continue
		}
		v, err := ParseValue(k, value)
		if err != nil {
			diags = append(diags, diag.New(diag.CodeSettingInvalid, loc, map[string]string{"key": name, "detail": err.Error()}))
			continue
		}
		lv.Values[k.Name] = v
	}
	return lv, diags
}

// ProcessEnviron returns the process environment. It is the module's only
// direct access to environment variables; callers pass the result down.
func ProcessEnviron() []string {
	return os.Environ()
}

func lookupEnv(environ []string, name string) string {
	for _, entry := range environ {
		if k, v, ok := strings.Cut(entry, "="); ok && k == name {
			return v
		}
	}
	return ""
}
