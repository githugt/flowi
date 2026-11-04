// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/discover"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

// DiscoverTimeout bounds one request of a discovery: a server that does not answer is a finding, not a hang.
const DiscoverTimeout = 15 * time.Second

// DiscoverPlan lists the requests a discovery of base would start with, sending nothing.
func (w *Workspace) DiscoverPlan(base string) ([]discover.Probe, error) {
	return discover.Plan(base, discover.DefaultPolicy())
}

// Discover probes base as discover.Run does, with the default policy, at most perSecond requests a second (0 keeps
// the default pace), trying only paths when it is not nil.
func (w *Workspace) Discover(ctx context.Context, base string, auth discover.Auth, perSecond int, paths []string) (discover.Report, error) {
	p := discover.DefaultPolicy()
	if perSecond > 0 {
		p.Interval = time.Second / time.Duration(perSecond)
	}
	return discover.Run(ctx, &http.Client{Timeout: DiscoverTimeout}, base, auth, p, paths)
}

// DiscoveredRequests are the requests SaveDiscovery made.
type DiscoveredRequests struct {
	Created []RequestTarget
	// Skipped says, one line per finding, why no request was made for it.
	Skipped []string
}

// SaveDiscovery drafts a request for every finding of rep that answered with JSON, in the discovered/ folder of the
// collection: a GET of base+path that expects the status it got, with the response's shape inferred from the body.
// Findings that are not JSON, were not a success, or are an API description (import that instead) are listed as
// skipped. A request that already exists is skipped too, so running it twice adds only what is new.
func (w *Workspace) SaveDiscovery(collection model.ID, base string, rep discover.Report) (DiscoveredRequests, error) {
	root, err := url.Parse(base)
	if err != nil || root.Host == "" {
		return DiscoveredRequests{}, fmt.Errorf("%q is not a URL", base)
	}
	var out DiscoveredRequests
	for _, f := range rep.Findings {
		if why := notDrafted(f); why != "" {
			out.Skipped = append(out.Skipped, f.Path+": "+why)
			continue
		}
		t, err := w.draftFinding(collection, root, f)
		if err != nil {
			out.Skipped = append(out.Skipped, f.Path+": "+err.Error())
			continue
		}
		out.Created = append(out.Created, t)
	}
	return out, nil
}

func notDrafted(f discover.Finding) string {
	switch {
	case f.Kind == "spec":
		return "an API description: import it instead of guessing"
	case f.Kind != "json":
		return "not JSON (status " + strconv.Itoa(f.Status) + ")"
	}
	return ""
}

func (w *Workspace) draftFinding(collection model.ID, root *url.URL, f discover.Finding) (RequestTarget, error) {
	v := model.RequestVersion{
		Label: "1", Method: "GET", URL: strings.TrimSuffix(root.String(), "/") + f.Path,
		Assert: []string{"status == " + strconv.Itoa(f.Status)},
	}
	t, err := w.newRequestWith(collection, discoveredName(f.Path), v)
	if err != nil {
		return RequestTarget{}, err
	}
	_, r, err := w.requestFor(t)
	if err != nil {
		return RequestTarget{}, err
	}
	if err := writeShallowSchema(r.File, SchemaPart{Kind: "response", Status: f.Status}, f.Sample); err != nil {
		return RequestTarget{}, fmt.Errorf("storing the shape of %s: %w", f.Path, err)
	}
	return t, nil
}

// discoveredName turns "/v1/pets/1" into "discovered/v1-pets-1" and the root into "discovered/root".
func discoveredName(path string) string {
	flat := strings.Trim(strings.ReplaceAll(path, "/", "-"), "-")
	if flat == "" {
		flat = "root"
	}
	var b strings.Builder
	for _, r := range flat {
		if r == '.' || r == '_' || r == '-' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	name := b.String()
	if len(name) > 60 {
		name = name[:60]
	}
	return "discovered/" + strings.TrimLeft(name, ".")
}

// writeShallowSchema stores the shape of one discovered answer, marked shallow: a single probe says little about
// optional fields and empty arrays, and the schema should not pass for a baseline until something confirms it.
func writeShallowSchema(requestFile string, part SchemaPart, sample any) error {
	file, err := schemaFile(requestFile, "1", part)
	if err != nil {
		return err
	}
	s := schemainfer.Infer(sample)
	s[schemaShallowKey] = true
	return writeSchema(file, s)
}

// reflectionServicePrefix starts the names of the services a server answers schema questions with; they are not an API to draft requests for.
const reflectionServicePrefix = "grpc.reflection."

// IsGRPCAddress says whether base names a gRPC server (grpc:// or grpcs://). Such a server is asked by reflection
// (DiscoverGRPC); anything else is probed with GET (Discover).
func IsGRPCAddress(base string) bool {
	return strings.HasPrefix(base, "grpc://") || strings.HasPrefix(base, "grpcs://")
}

// DiscoverGRPC asks the gRPC server at target (grpc://host:port, grpcs://host:port or host:port) which services and
// methods it offers, by reflection. Nothing is called: reflection only describes. A server with no reflection is
// reported as such, with the advice to add .proto files.
func (w *Workspace) DiscoverGRPC(ctx context.Context, target string) ([]grpcx.Method, error) {
	methods, err := grpcx.Methods(ctx, grpcx.Call{Server: target, Timeout: DiscoverTimeout})
	if err != nil {
		return nil, fmt.Errorf("asking %s what it offers: %w", target, err)
	}
	return callableMethods(methods), nil
}

func callableMethods(methods []grpcx.Method) []grpcx.Method {
	return slices.DeleteFunc(slices.Clone(methods), func(m grpcx.Method) bool {
		return strings.HasPrefix(m.Service, reflectionServicePrefix)
	})
}

// SaveGRPCDiscovery drafts a request for every method, in the discovered/ folder of the collection: a gRPC call to
// server with the method's example message, whose schema is asked of the server by reflection. A request that
// already exists is skipped, so running it twice adds only what is new. Messages that stream from the client start
// as an array of one example.
func (w *Workspace) SaveGRPCDiscovery(collection model.ID, server string, methods []grpcx.Method) (DiscoveredRequests, error) {
	if hostport, _ := model.GRPCTarget(server); hostport == "" {
		return DiscoveredRequests{}, fmt.Errorf("%q is not a gRPC server address", server)
	}
	var out DiscoveredRequests
	for _, m := range methods {
		t, err := w.newRequestWith(collection, discoveredGRPCName(m), grpcDraft(server, m))
		if err != nil {
			out.Skipped = append(out.Skipped, m.Full()+": "+err.Error())
			continue
		}
		out.Created = append(out.Created, t)
	}
	return out, nil
}

func grpcDraft(server string, m grpcx.Method) model.RequestVersion {
	g := model.GRPC{Server: server, Method: m.Full(), Body: model.Body{JSON: m.Example}}
	if m.ServerStream {
		g.Receive = &model.AsyncReceive{}
	}
	if m.ClientStream {
		g.Body = model.Body{JSON: "[" + m.Example + "]"}
	}
	return model.RequestVersion{Label: "1", GRPC: &g}
}

// discoveredGRPCName turns shop.Shop/Get into "discovered/grpc-shop.Shop-Get".
func discoveredGRPCName(m grpcx.Method) string {
	return "discovered/grpc-" + strings.Trim(m.Service, ".") + "-" + m.Name
}
