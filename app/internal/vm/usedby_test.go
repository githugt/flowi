// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func TestUsedBy_rows_per_version_and_consumer_targets(t *testing.T) {
	b := sampleBackend()
	b.ImpactReport = analysis.Report{Request: "auth/login", Status: analysis.Broken, Versions: []analysis.VersionUsage{
		{Version: "1", Contract: analysis.ContractInfo{Source: analysis.SourceExample}, Consumers: []analysis.Consumer{
			{Kind: analysis.KindNode, Collection: "Shop", CollectionID: "C1", Flow: "signup", Node: "login", Status: analysis.Broken,
				Reads: []analysis.Read{{Expr: "nodes.login.token", Target: "$.token", Status: analysis.Broken, Reason: "not in example 200.json"}}},
			{Kind: analysis.KindHook, Collection: "Shop", Hook: "post · collection Shop · inline #0", Status: analysis.Compatible},
		}},
	}}
	w := vmtest.SyncWorker()
	u := vm.NewUsedBy(b, w, flowi.RequestTarget{Collection: "C1", Request: "R1"})

	u.Load()
	w.Wait()

	rows := u.Rows()
	if len(rows) != 3 || rows[0].Header != "version 1 · contract: example" || rows[1].Text != "✗ flow signup › node login" ||
		rows[1].Detail != "nodes.login.token → $.token: not in example 200.json" || !rows[1].Opens || rows[2].Opens {
		t.Fatalf("rows %+v", rows)
	}
	if rows[1].Collection != "C1" || rows[1].Flow != "signup" || rows[1].Node != "login" {
		t.Fatalf("target %+v", rows[1])
	}
}
