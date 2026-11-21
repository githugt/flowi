// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Before: retries could not be written, and a nonsense value would have been taken at run time.
func TestCheckRetries(t *testing.T) {
	tests := []struct {
		name string
		kind model.NodeType // "" is a request step
		set  func(n *model.Node)
		want diag.Severity // 0 is no problem
	}{
		{name: "none"},
		{name: "a few", set: func(n *model.Node) { n.Retries = 3; n.RetryWait = time.Second }},
		{name: "the most", set: func(n *model.Node) { n.Retries = 10 }},
		{name: "too many", set: func(n *model.Node) { n.Retries = 11 }, want: diag.SeverityError},
		{name: "negative", set: func(n *model.Node) { n.Retries = -1 }, want: diag.SeverityError},
		{name: "negative wait", set: func(n *model.Node) { n.Retries = 1; n.RetryWait = -time.Second }, want: diag.SeverityError},
		{name: "wait alone", set: func(n *model.Node) { n.RetryWait = time.Second }, want: diag.SeverityWarning},
		{name: "on a note", kind: "note", set: func(n *model.Node) { n.Retries = 1 }, want: diag.SeverityError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, func(f *fixture, c *model.Collection, _ *model.Flow) {
				n := note("a")
				if tt.kind == "" {
					n = requestNode("a", f.request(c, "r", "1"))
					n.Request.Version = "1"
				}
				if tt.set != nil {
					tt.set(&n)
				}
				c.Flows[0].Nodes = []model.Node{n}
			})

			problems := slices.DeleteFunc(slices.Clone(got), func(d diag.Diagnostic) bool { return d.Code != diag.CodeNodeParam })
			switch {
			case tt.want == 0 && len(problems) != 0:
				t.Fatalf("unexpected %v", got)
			case tt.want != 0 && !hasDiag(problems, diag.CodeNodeParam, tt.want):
				t.Fatalf("want a %s of %s, got %v", tt.want, diag.CodeNodeParam, got)
			}
		})
	}
}
