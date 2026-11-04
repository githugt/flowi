// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestCompare_should_show_what_differs_between_two_environments(t *testing.T) {
	b := &vmtest.Backend{
		Cols: []model.Collection{vmtest.SampleCollection()},
		Comparison: flowi.EnvComparison{
			OnlyA:   []string{"debug"},
			Changed: []flowi.EnvChange{{Name: "base_url", A: "http://localhost", B: "https://api.example.com"}},
			Same:    3,
			Secrets: []string{"password"},
		},
	}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	got := app.Environments().Compare("dev", "prod")

	if got.A != "dev" || got.B != "prod" || len(got.OnlyA) != 1 || len(got.Changed) != 1 || got.Changed[0].B != "https://api.example.com" || got.Same != 3 {
		t.Fatalf("got %+v", got)
	}
	if got.OnlyB == nil || got.Secrets[0] != "password" {
		t.Fatalf("got %+v: lists must be empty, not null, and secrets named", got)
	}
}

func TestOverrides_should_count_what_an_environment_changes_about_the_one_it_extends(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Environments = []model.Environment{{Name: "base", File: "/e/base.yaml"}, {Name: "staging", File: "/e/staging.yaml", Extends: "base"}}
	b := &vmtest.Backend{Cols: []model.Collection{c}, Comparison: flowi.EnvComparison{
		Changed: []flowi.EnvChange{{Name: "base_url"}, {Name: "timeout"}}, OnlyB: []string{"debug"},
	}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())

	got := app.Environments().Overrides("staging")
	plain := app.Environments().Overrides("base")

	if got.Extends != "base" || got.Count != 3 || plain.Extends != "" || plain.Count != 0 {
		t.Fatalf("staging %+v, base %+v", got, plain)
	}
}
