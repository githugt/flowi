// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"reflect"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Before: a request could only write its own auth; it could not name one of the collection's.
func TestRequest_SetAuthUse_Given_an_inline_auth_When_a_named_one_is_picked_Then_the_version_only_names_it(t *testing.T) {
	r, _ := newRequestVM(t, &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}})
	r.SetAuthScheme(model.AuthDigest)
	r.SetAuthParam("username", "ada")

	r.SetAuthUse("partner-key")

	scheme, params := r.AuthScheme()
	if r.AuthUse() != "partner-key" || scheme != "" || len(params) != 0 || !r.Dirty() {
		t.Fatalf("use %q scheme %q params %v", r.AuthUse(), scheme, params)
	}
}

// Before: n/a (new): picking a scheme again must drop the name, or the file would carry both.
func TestRequest_SetAuthScheme_Given_a_named_auth_When_a_scheme_is_picked_Then_the_name_is_dropped(t *testing.T) {
	r, _ := newRequestVM(t, &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}})
	r.SetAuthUse("partner-key")

	r.SetAuthScheme(model.AuthDigest)

	if scheme, _ := r.AuthScheme(); r.AuthUse() != "" || scheme != model.AuthDigest {
		t.Fatalf("use %q scheme %q", r.AuthUse(), scheme)
	}
}

// Before: n/a (new): the file keeps parameters in the form's order, not the map's.
func TestOrderedAuthParams_Given_values_in_any_order_When_ordered_Then_scheme_parameters_come_first_and_empty_ones_are_left_out(t *testing.T) {
	values := map[string]string{"in": "header", "zz": "1", "value": "k", "name": "", "aa": "2"}

	got := vm.OrderedAuthParams(model.AuthAPIKey, values)

	want := []model.KeyValue{{Key: "value", Value: "k"}, {Key: "in", Value: "header"}, {Key: "aa", Value: "2"}, {Key: "zz", Value: "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
