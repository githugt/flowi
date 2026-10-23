// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"context"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

func generateSpec() Spec {
	return Spec{
		Type: "generate", Group: "Data", MapForm: true,
		Summary: "Generate values ($first-name, $uuid-v4, …) into variables; seedable.",
		Params:  []Param{{Name: "seed", Kind: ParamInt, Min: 1, Doc: "Seed for this node only, so its values do not change with the run seed."}},
		Run:     runGenerate,
	}
}

// runGenerate renders each "<var>: template" with the run's generators,
// or with its own seeded generators when seed is set.
func runGenerate(_ context.Context, n model.Node, s *State) Outcome {
	if seed, ok := param(n, "seed"); ok {
		s = withSeed(s, seed)
	}
	values, diags := mapForm(n, s, "seed")
	return writeVars(s, values, diags)
}

// withSeed is s with generators seeded by seed (an int64 from YAML).
func withSeed(s *State, seed any) *State {
	n, _ := seed.(int64)
	gen := template.NewGeneration(uint64(n), s.Runtime.Now).Func() //nolint:gosec // seeds are positive (validated); the bit pattern is all that matters
	seeded := *s
	inner := s.Scope
	seeded.Scope = func() template.Scope {
		sc := inner()
		sc.Gen = gen
		return sc
	}
	return &seeded
}
