// SPDX-License-Identifier: Apache-2.0

package synth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func namesOf(ovs []model.Overlay) []string {
	var out []string
	for _, o := range ovs {
		out = append(out, o.Name)
	}
	return out
}

func TestNegatives_should_drop_each_required_field_and_send_each_with_the_wrong_type(t *testing.T) {
	s := Subject{Request: "r1", Path: "pets/create", Body: map[string]any{"name": "Rex", "age": json.Number("3"), "vip": true}, Required: []string{"name", "age"}}

	got := Negatives(s)

	// Every field gets a wrong-type variant; only the required ones get a drop (vip is optional).
	want := "bad-age,bad-name,bad-vip,no-age,no-name"
	if strings.Join(namesOf(got), ",") != want {
		t.Fatalf("got %v want %s", namesOf(got), want)
	}
	drop := got[3].Requests[0]
	if drop.Request != "r1" || drop.Ops[0].Verb != model.OverlayMerge || drop.Ops[0].Path != "body.json" {
		t.Fatalf("op %+v", drop)
	}
	if patch := drop.Ops[0].Value.(map[string]any); patch["age"] != nil || len(patch) != 1 {
		t.Fatalf("a drop is a merge patch of null, got %v", patch)
	}
}

func TestNegatives_should_pick_a_wrong_value_of_another_type(t *testing.T) {
	s := Subject{Request: "r1", Body: map[string]any{"name": "Rex", "age": json.Number("3"), "vip": true, "tags": []any{"a"}}}

	got := Negatives(s)

	bad := map[string]any{}
	for _, o := range got {
		if f, ok := strings.CutPrefix(o.Name, "bad-"); ok {
			bad[f] = o.Requests[0].Ops[0].Value.(map[string]any)[f]
		}
	}
	if bad["name"] != 12345 || bad["age"] != "not-a-number" || bad["vip"] != "yes" || bad["tags"] != "oops" {
		t.Fatalf("wrong values %v", bad)
	}
}

func TestNegatives_Given_no_schema_Then_every_field_of_the_body_may_be_dropped(t *testing.T) {
	s := Subject{Request: "r1", Body: map[string]any{"a": "x", "b": "y"}}

	got := Negatives(s)

	if strings.Join(namesOf(got), ",") != "bad-a,bad-b,no-a,no-b" {
		t.Fatalf("got %v", namesOf(got))
	}
}

func TestNegatives_Given_an_authorization_header_Then_there_is_a_stale_token_variant(t *testing.T) {
	s := Subject{Request: "r1", Authorization: true}

	got := Negatives(s)

	if len(got) != 1 || got[0].Name != "stale-token" {
		t.Fatalf("got %v", namesOf(got))
	}
	op := got[0].Requests[0].Ops[0]
	value, _ := op.Value.(string)
	token, ok := strings.CutPrefix(value, "Bearer ")
	parts := strings.Split(token, ".")
	if op.Verb != model.OverlaySet || op.Path != "headers.Authorization" || !ok || len(parts) != 3 {
		t.Fatalf("op %+v", op)
	}
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	if string(payload) != `{"exp":1}` {
		t.Fatalf("payload %s: the token must have expired long ago", payload)
	}
}

func TestNegatives_should_give_every_variant_a_valid_overlay_name_even_for_odd_field_names(t *testing.T) {
	s := Subject{Request: "r1", Body: map[string]any{"user name": "x", "a/b": "y"}}

	got := Negatives(s)

	for _, o := range got {
		if strings.ContainsAny(o.Name, " /") {
			t.Fatalf("bad name %q", o.Name)
		}
	}
	if len(got) != 4 {
		t.Fatalf("got %v", namesOf(got))
	}
}

func TestNegatives_Given_nothing_to_break_Then_none(t *testing.T) {
	if got := Negatives(Subject{Request: "r1"}); len(got) != 0 {
		t.Fatalf("got %v", namesOf(got))
	}
}
