// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func statusCollection() model.Collection {
	return model.Collection{
		Folders: []model.Folder{
			{Dir: "requests/auth", Status: "in-development"},
			{Dir: "requests/auth/legacy", Status: "production"},
			{Dir: "requests/plain"},
			{Dir: "flows/nightly", Status: "testing", Tags: []string{"slow"}},
		},
		Requests: []model.Request{
			{Path: "auth/login"},
			{Path: "auth/refresh", Status: "review"},
			{Path: "auth/legacy/basic"},
			{Path: "plain/ping"},
			{Path: "top"},
		},
		Flows: []model.Flow{
			{Path: "nightly/sweep"},
			{Path: "nightly/deep/crawl"},
			{Path: "nightly/own", Status: "planning"},
			{Path: "adhoc"},
		},
	}
}

// Before: there was no status. The rule worth pinning is the one a folder's tags do not follow: tags add up, but an item
// has one status, so its own beats a folder's and the nearest folder beats one further up.
func TestRequestStatus_should_use_its_own_then_the_nearest_folder_above_then_none(t *testing.T) {
	c := statusCollection()

	cases := map[string]model.StatusOf{
		"auth/login":        {Name: "in-development", From: "requests/auth"},
		"auth/refresh":      {Name: "review"},
		"auth/legacy/basic": {Name: "production", From: "requests/auth/legacy"},
		"plain/ping":        {},
		"top":               {},
	}
	for path, want := range cases {
		i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.Path == path })

		if got := c.RequestStatus(c.Requests[i]); got != want {
			t.Errorf("%s: got %+v, want %+v", path, got, want)
		}
	}
}

func TestFlowStatus_should_reach_down_through_nested_folders(t *testing.T) {
	c := statusCollection()

	cases := map[string]model.StatusOf{
		"nightly/sweep":      {Name: "testing", From: "flows/nightly"},
		"nightly/deep/crawl": {Name: "testing", From: "flows/nightly"},
		"nightly/own":        {Name: "planning"},
		"adhoc":              {},
	}
	for path, want := range cases {
		i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.Path == path })

		if got := c.FlowStatus(c.Flows[i]); got != want {
			t.Errorf("%s: got %+v, want %+v", path, got, want)
		}
	}
}

func TestFolderStatus_should_be_its_own_or_come_from_a_folder_above_it(t *testing.T) {
	c := statusCollection()

	if got := c.FolderStatus("requests/auth/legacy"); got != (model.StatusOf{Name: "production"}) {
		t.Errorf("own: %+v", got)
	}
	if got := c.FolderStatus("requests/auth/legacy/deeper"); got != (model.StatusOf{Name: "production", From: "requests/auth/legacy"}) {
		t.Errorf("below a folder with a status, with no file of its own: %+v", got)
	}
	if got := c.FolderStatus("requests/plain"); got != (model.StatusOf{}) {
		t.Errorf("none: %+v", got)
	}
	if got := c.FolderStatus("requests"); got != (model.StatusOf{}) {
		t.Errorf("the top: %+v", got)
	}
}

func TestStatusList_should_hold_the_defaults_in_order_then_the_collections_own(t *testing.T) {
	m := model.Manifest{Statuses: []model.Status{{Name: "needs-design", Color: "pink"}}}

	var names []string
	for _, s := range m.StatusList() {
		names = append(names, s.Name)
	}

	if got := strings.Join(names, ","); got != "planning,in-development,review,testing,production,needs-design" {
		t.Fatalf("got %s", got)
	}
}

func TestStatusList_Given_a_default_s_name_Then_it_changes_that_default_in_place(t *testing.T) {
	m := model.Manifest{Statuses: []model.Status{{Name: "testing", Color: "red", Icon: "bug"}, {Name: "testing", Color: "teal"}}}

	got := m.StatusList()

	if len(got) != 5 || got[3] != (model.Status{Name: "testing", Color: "red", Icon: "bug"}) {
		t.Fatalf("got %+v (the first definition counts, the order stays)", got)
	}
}

func TestStatusList_should_fill_a_missing_colour_and_icon(t *testing.T) {
	c := model.Collection{Manifest: model.Manifest{Statuses: []model.Status{{Name: "later"}}}}

	got, ok := c.StatusNamed("later")

	if !ok || got.Color != "gray" || got.Icon != "flag" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if _, ok := c.StatusNamed("nope"); ok {
		t.Fatal("an unknown name was found")
	}
}

func TestDefaultStatuses_should_each_have_a_colour_and_an_icon_the_window_can_draw_and_no_two_alike(t *testing.T) {
	seenIcon, seenColor := map[string]bool{}, map[string]bool{}

	for _, s := range model.DefaultStatuses() {
		if err := model.CheckStatus(s); err != nil || s.Color == "" || s.Icon == "" {
			t.Errorf("%s: %v", s.Name, err)
		}
		if seenIcon[s.Icon] || seenColor[s.Color] {
			t.Errorf("%s repeats an icon or a colour, so two statuses would look alike", s.Name)
		}
		seenIcon[s.Icon], seenColor[s.Color] = true, true
	}
}

func TestCheckStatus_should_refuse_what_cannot_be_used(t *testing.T) {
	cases := []struct {
		s    model.Status
		want string
	}{
		{model.Status{}, "empty"},
		{model.Status{Name: "Review"}, "lower-case"},
		{model.Status{Name: "needs design"}, "no spaces"},
		{model.Status{Name: strings.Repeat("a", 41)}, "longer than"},
		{model.Status{Name: "ok", Color: "mauve"}, "not a status colour"},
		{model.Status{Name: "ok", Icon: "nope"}, "not a status icon"},
	}
	for _, c := range cases {
		err := model.CheckStatus(c.s)

		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%+v: got %v, want it to say %q", c.s, err, c.want)
		}
	}
	if err := model.CheckStatus(model.Status{Name: "needs-design_2", Color: "pink", Icon: "bookmark"}); err != nil {
		t.Errorf("a good status was refused: %v", err)
	}
}

func TestRequestsWithStatus_and_FlowsWithStatus_should_count_what_a_folder_passes_down(t *testing.T) {
	c := statusCollection()

	reqs := c.RequestsWithStatus("in-development")
	flows := c.FlowsWithStatus("testing")

	if len(reqs) != 1 || reqs[0].Path != "auth/login" || len(flows) != 2 {
		t.Fatalf("requests %v flows %v", reqs, flows)
	}
}

func TestCleanTags_should_make_one_word_tags_drop_empties_and_keep_the_first_of_a_repeat(t *testing.T) {
	got := model.CleanTags([]string{" smoke ", "needs review", "a,b", "", "smoke", "  ", "-x-"})

	if want := []string{"smoke", "needs-review", "a-b", "x"}; !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestTagsInUse_should_count_what_requests_flows_and_folders_carry_most_used_first(t *testing.T) {
	c := model.Collection{
		Folders:  []model.Folder{{Dir: "flows/x", Tags: []string{"slow"}}},
		Requests: []model.Request{{Tags: []string{"smoke", "auth"}}, {Tags: []string{"smoke"}}},
		Flows:    []model.Flow{{Tags: []string{"slow", "smoke"}}},
	}

	got := c.TagsInUse()

	want := []model.TagUse{{Tag: "smoke", Count: 3}, {Tag: "slow", Count: 2}, {Tag: "auth", Count: 1}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
}
