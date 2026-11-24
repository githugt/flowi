// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
)

// Before: a request could not change type, and its name could not be written on its screen.
func TestRequestService_SwitchType_Given_an_http_request_Then_the_view_has_the_new_type_and_a_notice_says_what_was_left(t *testing.T) {
	reqs, v, pub, _, _ := openLoginOptions(t)
	if v.Type != "http" || len(v.Types) != 5 {
		t.Fatalf("type %q, %d types", v.Type, len(v.Types))
	}

	reqs.SwitchType(v.Node, "message")

	got, ok := pub.last["request"].(bridge.RequestView)
	if !ok || got.Type != "message" || got.Kind != "async" || !got.Dirty {
		t.Fatalf("view %+v", pub.last["request"])
	}
	n, ok := pub.last["notice"].(bridge.Notice)
	if !ok || !strings.Contains(n.Text, "messaging") || !strings.Contains(n.Text, "Discard") {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
}

func TestRequestService_SwitchType_Given_an_unknown_type_Then_a_notice_refuses_it(t *testing.T) {
	reqs, v, pub, _, _ := openLoginOptions(t)

	reqs.SwitchType(v.Node, "carrier-pigeon")

	n, ok := pub.last["notice"].(bridge.Notice)
	if !ok || n.Level != "error" {
		t.Fatalf("notice %+v", pub.last["notice"])
	}
}

func TestRequestService_SetName_Then_the_view_carries_it_unsaved_and_the_five_types_are_named(t *testing.T) {
	reqs, v, pub, _, _ := openLoginOptions(t)

	reqs.SetName(v.Node, "Sign in")

	got := pub.last["request"].(bridge.RequestView)
	if got.Name != "Sign in" || !got.Dirty {
		t.Fatalf("name %q dirty %v", got.Name, got.Dirty)
	}
	var labels []string
	for _, c := range got.Types {
		labels = append(labels, c.Label)
		if c.Icon == "" {
			t.Errorf("%s has no icon", c.ID)
		}
	}
	if strings.Join(labels, ",") != "HTTP,gRPC,Messaging,Database,Data file" {
		t.Fatalf("labels %v", labels)
	}
}

func TestRequestService_SetName_Then_the_tree_shows_the_typed_name_and_clearing_it_shows_the_file_name_again(t *testing.T) {
	reqs, v, _, app, _ := openLoginOptions(t)
	label := func() string {
		for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
			for _, c := range n.Children {
				if c.ID == v.Node {
					return c.Label
				}
			}
		}
		return ""
	}

	reqs.SetName(v.Node, "Sign in")
	typed := label()
	reqs.SetName(v.Node, "")
	cleared := label()

	if typed != "Sign in" || cleared != "login" {
		t.Fatalf("typed %q, cleared %q", typed, cleared)
	}
}
