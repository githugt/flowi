// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"errors"
	"sync"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/discover"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// ProbeView is one request a discovery would make: a GET of Path, or (Method "reflect") asking a gRPC server which
// services it offers.
type ProbeView struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Why    string `json:"why"`
}

// FindingView is what one probed path answered.
type FindingView struct {
	Path        string `json:"path"`
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Kind        string `json:"kind"` // spec, json or other; grpc for a method found by reflection
}

// DiscoveryView is what a discovery found, and what saving it made.
type DiscoveryView struct {
	Findings []FindingView `json:"findings"`
	Stopped  string        `json:"stopped"`
	Created  int           `json:"created"`
	Skipped  []string      `json:"skipped"`
}

// discovered is the last discovery, kept so saving it does not ask the server again.
type discovered struct {
	mu      sync.Mutex
	base    string
	rep     discover.Report
	methods []grpcx.Method // set instead of rep when base is a gRPC server
}

func (d *discovered) set(base string, rep discover.Report, methods []grpcx.Method) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.base, d.rep, d.methods = base, rep, methods
}

func (d *discovered) get() (string, discover.Report, []grpcx.Method) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.base, d.rep, d.methods
}

// DiscoverPlan lists the requests a discovery of base starts with. Nothing is sent: the person ticks the ones
// to make before Discover runs.
func (s *ImportService) DiscoverPlan(base string) ([]ProbeView, error) {
	if flowi.IsGRPCAddress(base) {
		return []ProbeView{{Method: "reflect", Path: base, Why: "asks which services it offers; no method is called"}}, nil
	}
	plan, err := s.app.backend.DiscoverPlan(base)
	if err != nil {
		return nil, err
	}
	out := make([]ProbeView, 0, len(plan))
	for _, p := range plan {
		out = append(out, ProbeView{Method: "GET", Path: p.Path, Why: p.Why})
	}
	return out, nil
}

// Discover sends the ticked paths of base, and the same-host paths their answers point to, and says what answered.
// header and token are sent to base only and are not kept; a token alone goes as a Bearer token in Authorization.
func (s *ImportService) Discover(base, header, token string, paths []string) (DiscoveryView, error) {
	if len(paths) == 0 {
		return DiscoveryView{}, errors.New("tick at least one request to make")
	}
	if flowi.IsGRPCAddress(base) {
		return s.discoverGRPC(base)
	}
	auth := discover.Auth{}
	if token != "" {
		auth = discover.Auth{Header: header, Value: token}
		if header == "" {
			auth = discover.Auth{Header: "Authorization", Value: "Bearer " + token}
		}
	}
	rep, err := s.app.backend.Discover(context.Background(), base, auth, 0, paths)
	if err != nil {
		return DiscoveryView{}, err
	}
	s.found.set(base, rep, nil)
	return discoveryViewOf(rep, 0, nil), nil
}

// SaveDiscovery drafts requests in a collection from the last discovery, under discovered/.
func (s *ImportService) SaveDiscovery(collection string) (DiscoveryView, error) {
	base, rep, methods := s.found.get()
	if base == "" {
		return DiscoveryView{}, errors.New("run a discovery first")
	}
	if flowi.IsGRPCAddress(base) {
		return s.saveGRPCDiscovery(collection, base, methods)
	}
	saved, err := s.app.backend.SaveDiscovery(model.ID(collection), base, rep)
	if err != nil {
		return DiscoveryView{}, vmError(err)
	}
	return discoveryViewOf(rep, len(saved.Created), saved.Skipped), nil
}

// discoverGRPC asks the server which methods it offers; each shows as a finding of kind grpc.
func (s *ImportService) discoverGRPC(server string) (DiscoveryView, error) {
	methods, err := s.app.backend.DiscoverGRPC(context.Background(), server)
	if err != nil {
		return DiscoveryView{}, vmError(err)
	}
	s.found.set(server, discover.Report{}, methods)
	out := DiscoveryView{Findings: []FindingView{}, Skipped: []string{}}
	for _, m := range methods {
		out.Findings = append(out.Findings, FindingView{Path: m.Full(), Kind: "grpc"})
	}
	return out, nil
}

func (s *ImportService) saveGRPCDiscovery(collection, server string, methods []grpcx.Method) (DiscoveryView, error) {
	saved, err := s.app.backend.SaveGRPCDiscovery(model.ID(collection), server, methods)
	if err != nil {
		return DiscoveryView{}, vmError(err)
	}
	out := DiscoveryView{Findings: []FindingView{}, Created: len(saved.Created), Skipped: append([]string{}, saved.Skipped...)}
	for _, m := range methods {
		out.Findings = append(out.Findings, FindingView{Path: m.Full(), Kind: "grpc"})
	}
	return out, nil
}

func vmError(err error) error { return errors.New(vm.ErrorText(err)) }

func discoveryViewOf(rep discover.Report, created int, skipped []string) DiscoveryView {
	out := DiscoveryView{Findings: []FindingView{}, Stopped: rep.Stopped, Created: created, Skipped: append([]string{}, skipped...)}
	for _, f := range rep.Findings {
		out.Findings = append(out.Findings, FindingView{Path: f.Path, Status: f.Status, ContentType: f.ContentType, Kind: f.Kind})
	}
	return out
}
