// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"

	"github.com/githugt/flowi/lib/analysis"
)

func TestNewlyBroken_Given_consumers_broken_only_by_the_edit_Then_only_those_are_listed(t *testing.T) {
	flow := func(node string, s analysis.Status) analysis.Consumer {
		return analysis.Consumer{Kind: analysis.KindNode, File: "f", Flow: "signup", Node: node, Version: "1", Status: s,
			Reads: []analysis.Read{{Expr: "{{extract.token}}", Target: "token", Status: s, Reason: "no extraction called token"}}}
	}
	saved := analysis.Report{Versions: []analysis.VersionUsage{{Version: "1", Consumers: []analysis.Consumer{flow("a", analysis.Compatible), flow("b", analysis.Broken)}}}}
	edited := analysis.Report{Versions: []analysis.VersionUsage{{Version: "1", Consumers: []analysis.Consumer{flow("a", analysis.Broken), flow("b", analysis.Broken)}}}}

	got := newlyBroken(saved, edited)

	if len(got) != 1 || got[0] != "flow signup › node a (version 1): {{extract.token}} → token: no extraction called token" {
		t.Fatalf("got %q", got)
	}
}
