// SPDX-License-Identifier: Apache-2.0

package importers

import (
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// EnvVar is an imported environment variable: a credential-like one (by name,
// or because the source marks it secret) is declared as a secret without its
// value, which would otherwise end up in a committed file.
func EnvVar(it *Item, name, value string, secret bool) model.EnvVar {
	if secret || SensitiveHeader(name) {
		it.Report(diag.New(diag.CodeImportSensitive, diag.Location{}, map[string]string{
			"where": "variable " + name, "placeholder": "a secret declaration"}))
		return model.EnvVar{Name: name, Secret: true}
	}
	text, _ := Translate(value)
	return model.EnvVar{Name: name, Value: text}
}
