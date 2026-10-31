// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"regexp"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// CollectionVariables lists the variables of a collection, in file order.
func (w *Workspace) CollectionVariables(collection model.ID) ([]model.KeyValue, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	return append([]model.KeyValue(nil), c.Manifest.Variables...), nil
}

// SetCollectionVariables replaces the variables of a collection, written to
// its flowi.yaml. Requests and flows read them as {{collection_var.name}}; the
// selected environment overrides one of the same name. A name is letters,
// digits and underscores, and a name given twice is refused.
func (w *Workspace) SetCollectionVariables(collection model.ID, vars []model.KeyValue, o SaveOptions) error {
	if err := checkVariableNames(vars); err != nil {
		return err
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	man := c.Manifest
	man.Variables = append([]model.KeyValue(nil), vars...)
	data, err := w.store.EncodeManifest(man)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", man.File, err)
	}
	return w.writeAndReload(collection, man.File, data, o)
}

var variableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func checkVariableNames(vars []model.KeyValue) error {
	seen := map[string]bool{}
	for _, kv := range vars {
		switch {
		case !variableName.MatchString(kv.Key):
			return fmt.Errorf("%q is not a variable name: use letters, digits and underscores, not starting with a digit", kv.Key)
		case seen[kv.Key]:
			return fmt.Errorf("the variable %q is given twice", kv.Key)
		}
		seen[kv.Key] = true
	}
	return nil
}
