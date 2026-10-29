// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func TestSetCollectionVariables_should_write_them_to_the_manifest_and_read_them_back(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)

	// Act
	err := w.SetCollectionVariables(target.Collection, []model.KeyValue{{Key: "region", Value: "eu"}, {Key: "api_base", Value: "https://api.example.com"}}, SaveOptions{DropComments: true})
	got, readErr := w.CollectionVariables(target.Collection)

	// Assert
	if err != nil || readErr != nil || len(got) != 2 || got[0].Key != "region" || got[1].Value != "https://api.example.com" {
		t.Fatalf("got %+v, err %v / %v", got, err, readErr)
	}
}

func TestSetCollectionVariables_should_refuse_a_name_that_cannot_be_read_in_a_template_and_a_repeat(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)
	bad := [][]model.KeyValue{
		{{Key: "with space", Value: "x"}},
		{{Key: "1st", Value: "x"}},
		{{Key: "a.b", Value: "x"}},
		{{Key: "", Value: "x"}},
		{{Key: "dup", Value: "1"}, {Key: "dup", Value: "2"}},
	}
	for _, vars := range bad {
		if err := w.SetCollectionVariables(target.Collection, vars, SaveOptions{DropComments: true}); err == nil {
			t.Errorf("%v was accepted", vars)
		}
	}
}
