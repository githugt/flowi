// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"path/filepath"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// grpcDiags reports what is wrong with a gRPC version; path is the path of
// its grpc block, and alsoAsync says the version has an async block too.
func grpcDiags(file, path string, alsoAsync bool, g model.GRPC) diag.List {
	at := func(p string) diag.Location { return diag.Location{File: file, Path: p} }
	var out diag.List
	problem := func(p, detail string) {
		out = append(out, diag.New(diag.CodeGRPCRequest, at(p), map[string]string{"detail": detail}))
	}
	if alsoAsync {
		problem(path, "a version is either a gRPC call or an async request, not both")
	}
	if detail := grpcServerProblem(g.Server, g.Web); detail != "" {
		problem(yamlpath.Key(path, "server"), detail)
	}
	if g.Web && len(g.Proto) == 0 {
		problem(yamlpath.Key(path, "proto"), "a gRPC-Web call needs the service's .proto files: its proxy does not offer reflection")
	}
	if !templated(g.Method) && g.Method != "" {
		if _, _, ok := model.SplitMethod(g.Method); !ok {
			problem(yamlpath.Key(path, "method"), "the method is written package.Service/Method")
		}
	}
	if len(g.Body.Multipart) > 0 {
		problem(yamlpath.Key(path, "body"), "a gRPC message is JSON; a multipart body cannot be sent")
	}
	if r := g.Receive; r != nil {
		recv := yamlpath.Key(path, "receive")
		if r.Count < 0 {
			problem(yamlpath.Key(recv, "count"), "count must not be negative")
		}
		if r.Timeout < 0 {
			problem(yamlpath.Key(recv, "timeout"), "timeout must not be negative")
		}
	}
	return append(out, grpcPathDiags(at, path, g)...)
}

// grpcServerProblem is what is wrong with a server, "" when it is fine. A
// templated server is only known when it runs.
func grpcServerProblem(server string, web bool) string {
	if server == "" || strings.HasPrefix(strings.TrimSpace(server), "{{") {
		return ""
	}
	if web {
		if scheme, _, found := strings.Cut(server, "://"); found && !strings.EqualFold(scheme, "http") && !strings.EqualFold(scheme, "https") {
			return "a gRPC-Web server is an http:// or https:// address"
		}
		return ""
	}
	if scheme, _, found := strings.Cut(server, "://"); found &&
		!strings.EqualFold(scheme, model.GRPCSchemePlaintext) && !strings.EqualFold(scheme, model.GRPCSchemeTLS) {
		return "the server is grpc://host:port, grpcs://host:port or host:port"
	}
	return ""
}

// grpcPathDiags checks that the .proto files and import roots stay inside the
// collection: a request file must not read an arbitrary path of the machine.
func grpcPathDiags(at func(string) diag.Location, path string, g model.GRPC) diag.List {
	var out diag.List
	check := func(key string, paths []string) {
		for i, p := range paths {
			if templated(p) || filepath.IsLocal(p) {
				continue
			}
			out = append(out, diag.New(diag.CodeGRPCProto, at(yamlpath.Index(yamlpath.Key(path, key), i)),
				map[string]string{"detail": key + " path " + p + " must be a relative path inside the collection"}))
		}
	}
	check("proto", g.Proto)
	check("import", g.Import)
	return out
}
