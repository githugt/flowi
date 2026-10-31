// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
)

type fakeVariables struct {
	rows []model.KeyValue
	err  error
	drop bool // what the last write was allowed
}

func (f *fakeVariables) CollectionVariables(model.ID) ([]model.KeyValue, error) { return f.rows, nil }

func (f *fakeVariables) SetCollectionVariables(_ model.ID, vars []model.KeyValue, o flowi.SaveOptions) error {
	f.drop = o.DropComments
	if f.err != nil && !o.DropComments {
		return f.err
	}
	f.rows = vars
	return nil
}

func TestManagerService_Variables_should_list_save_and_ask_before_dropping_comments(t *testing.T) {
	// Arrange
	fake := &fakeVariables{rows: []model.KeyValue{{Key: "region", Value: "eu"}}, err: diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "flowi.yaml"})}.Err()}
	_, _, _, app, _ := openLoginOptions(t, bridge.WithCollectionVariables(fake))
	manager := app.Manager()

	// Act
	listed, listErr := manager.Variables("c")
	asked, askErr := manager.SetVariables("c", []model.KeyValue{{Key: "region", Value: "us"}}, false)
	saved, saveErr := manager.SetVariables("c", []model.KeyValue{{Key: "region", Value: "us"}}, true)

	// Assert
	if listErr != nil || len(listed) != 1 || listed[0].Key != "region" {
		t.Fatalf("listed %v err %v", listed, listErr)
	}
	if !asked || askErr != nil {
		t.Errorf("a file with comments must be asked about first: asked %v err %v", asked, askErr)
	}
	if saved || saveErr != nil || fake.rows[0].Value != "us" || !fake.drop {
		t.Errorf("after the answer: needs %v err %v rows %v", saved, saveErr, fake.rows)
	}
}

func TestManagerService_Variables_Given_no_support_Then_lists_none_and_refuses_to_save(t *testing.T) {
	_, _, _, app, _ := openLoginOptions(t)
	manager := app.Manager()

	listed, listErr := manager.Variables("c")
	_, saveErr := manager.SetVariables("c", nil, false)

	if listErr != nil || listed == nil || len(listed) != 0 || saveErr == nil {
		t.Fatalf("listed %v err %v / %v", listed, listErr, saveErr)
	}
}
