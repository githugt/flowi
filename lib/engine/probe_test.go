// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"
	"sync"

	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
)

// probe is a test node type: it records when each node instance starts
// and ends (as a logical sequence), and runs a per-node behaviour.
type probe struct {
	mu       sync.Mutex
	seq      int
	starts   map[string]int
	ends     map[string]int
	order    []string
	inflight int
	peak     int
	behave   map[string]func(ctx context.Context, s *nodes.State) nodes.Outcome
}

func newProbe() *probe {
	return &probe{starts: map[string]int{}, ends: map[string]int{}, behave: map[string]func(context.Context, *nodes.State) nodes.Outcome{}}
}

func (p *probe) run(ctx context.Context, n model.Node, s *nodes.State) nodes.Outcome {
	p.mark(s.Path, true)
	defer p.mark(s.Path, false)
	if b, ok := p.behave[n.ID]; ok {
		return b(ctx, s)
	}
	return nodes.Outcome{}
}

func (p *probe) mark(path string, start bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	if start {
		if _, again := p.starts[path]; again {
			panic("node started twice: " + path)
		}
		p.starts[path] = p.seq
		p.order = append(p.order, "start:"+path)
		p.inflight++
		p.peak = max(p.peak, p.inflight)
		return
	}
	p.ends[path] = p.seq
	p.order = append(p.order, "end:"+path)
	p.inflight--
}

func (p *probe) started(path string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.starts[path]
	return ok
}

func (p *probe) deps(d Deps) Deps {
	return withRunners(d, map[model.NodeType]nodes.Runner{"probe": p.run})
}

// failing is a behaviour that fails the node.
func failing(context.Context, *nodes.State) nodes.Outcome {
	return nodes.Outcome{Status: nodes.StatusFailed, Diags: assertDiag()}
}

func probeNode(id string, needs ...string) model.Node {
	return model.Node{ID: id, Type: "probe", Needs: needs}
}

func concurrentNode(id string, needs ...string) model.Node {
	n := probeNode(id, needs...)
	n.Concurrent = true
	return n
}

func (p *probe) String() string { return fmt.Sprint(p.order) }
