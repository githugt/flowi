// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"reflect"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func inline(id string) model.HookStep {
	return model.HookStep{Inline: []model.Node{{ID: id, Type: "log"}}}
}

func hookIDs(refs []HookRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Level + ":" + r.Step.Inline[0].ID
	}
	return out
}

func layeredCollection() (model.Collection, model.Request, model.RequestVersion) {
	v := model.RequestVersion{Label: "1", Hooks: model.Hooks{Pre: []model.HookStep{inline("v-pre")}, Post: []model.HookStep{inline("v-post")}}}
	r := model.Request{Path: "a/b/login", Versions: []model.RequestVersion{v},
		Hooks: model.Hooks{Pre: []model.HookStep{inline("r-pre")}, Post: []model.HookStep{inline("r-post")}}}
	c := model.Collection{
		Manifest: model.Manifest{Hooks: model.Hooks{Pre: []model.HookStep{inline("c-pre")}, Post: []model.HookStep{inline("c-post")}}},
		Folders: []model.Folder{
			{Dir: "requests/a/b", Hooks: model.Hooks{Pre: []model.HookStep{inline("ab-pre")}, Post: []model.HookStep{inline("ab-post")}}},
			{Dir: "requests/other", Hooks: model.Hooks{Pre: []model.HookStep{inline("other-pre")}}},
			{Dir: "requests/a", Hooks: model.Hooks{Pre: []model.HookStep{inline("a-pre")}, Post: []model.HookStep{inline("a-post")}}},
			{Dir: "requests", Hooks: model.Hooks{Pre: []model.HookStep{inline("root-pre")}}},
		},
		Requests: []model.Request{r},
	}
	return c, r, v
}

func TestChainFor_should_order_collection_folders_request_version_and_reverse_post(t *testing.T) {
	c, r, v := layeredCollection()

	chain := ChainFor(c, r, v)

	wantPre := []string{"collection:c-pre", "folder:root-pre", "folder:a-pre", "folder:ab-pre", "request:r-pre", "version:v-pre"}
	wantPost := []string{"version:v-post", "request:r-post", "folder:ab-post", "folder:a-post", "collection:c-post"}
	if got := hookIDs(chain.Pre); !reflect.DeepEqual(got, wantPre) {
		t.Fatalf("pre %v", got)
	}
	if got := hookIDs(chain.Post); !reflect.DeepEqual(got, wantPost) {
		t.Fatalf("post %v", got)
	}
}

func TestHookChain_Filter(t *testing.T) {
	c, r, v := layeredCollection()
	chain := ChainFor(c, r, v)
	tests := []struct {
		ctl       model.HookControl
		pre, post int
	}{
		{model.HookControl{}, 6, 5},
		{model.HookControl{Mode: "all"}, 6, 5},
		{model.HookControl{Mode: "none"}, 0, 0},
		{model.HookControl{Skip: []string{"pre"}}, 0, 5},
		{model.HookControl{Skip: []string{"post"}}, 6, 0},
		{model.HookControl{Skip: []string{"pre", "post"}}, 0, 0},
	}
	for _, tt := range tests {
		got := chain.Filter(tt.ctl)

		if len(got.Pre) != tt.pre || len(got.Post) != tt.post {
			t.Errorf("%+v: pre %d post %d", tt.ctl, len(got.Pre), len(got.Post))
		}
	}
}

func TestHookRef_Label_should_name_flow_path_or_inline(t *testing.T) {
	flow := HookRef{Step: model.HookStep{Flow: &model.Ref{ID: "01X", Path: "hooks/sign"}}}
	in := HookRef{Step: inline("x")}

	if flow.Label() != "hooks/sign" || in.Label() != "inline" {
		t.Fatalf("labels %q %q", flow.Label(), in.Label())
	}
}
