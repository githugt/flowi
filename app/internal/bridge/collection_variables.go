// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// CollectionVariables is the library's collection variables, kept apart as the
// workspace file and the spec overlays are.
type CollectionVariables interface {
	CollectionVariables(collection model.ID) ([]model.KeyValue, error)
	SetCollectionVariables(collection model.ID, vars []model.KeyValue, o flowi.SaveOptions) error
}

// WithCollectionVariables lets the window read and change the variables of a collection.
func WithCollectionVariables(v CollectionVariables) Option {
	return func(o *appOptions) { o.variables = v }
}

var errNoVariables = errors.New("collection variables are not available here")

// Variables lists the variables of a collection, read in templates as collection_var.<name>.
func (s *ManagerService) Variables(collection string) ([]model.KeyValue, error) {
	if s.app.variables == nil {
		return []model.KeyValue{}, nil
	}
	var rows []model.KeyValue
	var err error
	s.app.actor.Do(func() { rows, err = s.app.variables.CollectionVariables(model.ID(collection)) })
	if rows == nil {
		rows = []model.KeyValue{}
	}
	return rows, err
}

// SetVariables replaces the variables of a collection. needsConfirm is true when
// the file has comments that writing would drop: ask, then call again with dropComments.
func (s *ManagerService) SetVariables(collection string, rows []model.KeyValue, dropComments bool) (needsConfirm bool, err error) {
	if s.app.variables == nil {
		return false, errNoVariables
	}
	s.app.actor.Do(func() {
		err = s.app.variables.SetCollectionVariables(model.ID(collection), rows, flowi.SaveOptions{DropComments: dropComments})
	})
	if vm.IsDropsComments(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("saving the variables: %w", err)
	}
	return false, nil
}
