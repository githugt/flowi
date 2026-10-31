// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

// randomList builds a node list: random needs to earlier siblings, random
// concurrent flags and random failures. It returns the nodes and which
// ones fail.
func randomList(rng *rand.Rand) ([]model.Node, map[string]bool) {
	count := 2 + rng.IntN(9)
	list := make([]model.Node, count)
	fails := map[string]bool{}
	for i := range list {
		id := fmt.Sprintf("n%d", i)
		n := probeNode(id)
		if i > 0 && rng.Float64() < 0.4 {
			for j := range i {
				if rng.Float64() < 0.4 {
					n.Needs = append(n.Needs, list[j].ID)
				}
			}
		}
		n.Concurrent = rng.Float64() < 0.4
		fails[id] = rng.Float64() < 0.1
		list[i] = n
	}
	return list, fails
}

func depsOf(list []model.Node, i int) []string {
	if len(list[i].Needs) > 0 {
		return list[i].Needs
	}
	if i == 0 {
		return nil
	}
	return []string{list[i-1].ID}
}

func TestScheduler_property_random_graphs(t *testing.T) {
	policies := []string{"fail_fast", "finish_running", "continue_all"}
	for seed := uint64(1); seed <= 300; seed++ {
		rng := rand.New(rand.NewPCG(seed, 7))
		list, fails := randomList(rng)
		policy := policies[rng.IntN(len(policies))]
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			f := newFixture(t)
			p := newProbe()
			anyFailure := false
			for id, fail := range fails {
				if fail {
					anyFailure = true
					p.behave[id] = failing
				}
			}
			fl := f.addFlow(fmt.Sprintf("prop-%d", seed), list...)
			fl.Execution.OnBranchError = policy

			_, res := runFlow(testCtx(t), t, p.deps(f.deps()), f.runInput(fl))

			checkSchedule(t, seed, policy, list, fails, p)
			if !anyFailure && (res.Status != StatusPassed || len(p.starts) != len(list)) {
				t.Fatalf("seed %d: no failures but status %s, ran %d/%d (%v)", seed, res.Status, len(p.starts), len(list), p.order)
			}
		})
	}
}

func checkSchedule(t *testing.T, seed uint64, policy string, list []model.Node, fails map[string]bool, p *probe) {
	t.Helper()
	for i, n := range list {
		start, ran := p.starts[n.ID]
		if !ran {
			continue
		}
		for _, d := range depsOf(list, i) {
			end, depRan := p.ends[d]
			if !depRan || end > start {
				t.Fatalf("seed %d (%s): %s started before dependency %s finished (%v)", seed, policy, n.ID, d, p.order)
			}
			if fails[d] {
				t.Fatalf("seed %d (%s): %s ran although dependency %s failed (%v)", seed, policy, n.ID, d, p.order)
			}
		}
		if n.Concurrent {
			continue
		}
		for _, earlier := range list[:i] {
			if end, ok := p.ends[earlier.ID]; ok && end > start {
				t.Fatalf("seed %d (%s): serial %s started before earlier %s finished (%v)", seed, policy, n.ID, earlier.ID, p.order)
			}
			if _, started := p.starts[earlier.ID]; started && !slices.Contains(p.order, "end:"+earlier.ID) {
				t.Fatalf("seed %d: %s never ended", seed, earlier.ID)
			}
		}
	}
}
