// SPDX-License-Identifier: Apache-2.0

package speclint

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/githugt/flowi/lib/diag"
)

// LintProto checks the services of a .proto file the way the other descriptions are checked: info-description asks
// that each service say what it is for, operation-description that each rpc say what it does, both by the comment
// written above it. data is the file; name is how other files import it and read finds the files it imports (nil when
// it imports none); file is where the findings point, and they carry the line of the service or rpc, since a .proto
// file has no YAML path. A file that does not compile is left alone: the import said so already.
func LintProto(name string, data []byte, read func(path string) ([]byte, error), file string, cfg Config) diag.List {
	resolver := protocompile.WithStandardImports(&protocompile.SourceResolver{Accessor: func(p string) (io.ReadCloser, error) {
		if p == name {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
		if read == nil {
			return nil, os.ErrNotExist
		}
		b, err := read(p)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}})
	compiled, err := (&protocompile.Compiler{Resolver: resolver, SourceInfoMode: protocompile.SourceInfoStandard}).Compile(context.Background(), name)
	if err != nil || len(compiled) == 0 {
		return nil
	}
	return protoFindings(compiled[0], name, file, cfg)
}

func protoFindings(fd linker.File, spec, file string, cfg Config) diag.List {
	var out diag.List
	add := func(r rule, f finding) {
		out = append(out, finish(r, levelOf(r, cfg), spec, file, f))
	}
	rs := map[string]rule{}
	for _, r := range rules() {
		if levelOf(r, cfg) != LevelOff {
			rs[r.name] = r
		}
	}
	commented := func(d protoreflect.Descriptor) (line int, ok bool) {
		loc := fd.SourceLocations().ByDescriptor(d)
		return loc.StartLine + 1, strings.TrimSpace(loc.LeadingComments+loc.TrailingComments) != ""
	}
	services := fd.Services()
	for i := range services.Len() {
		svc := services.Get(i)
		if r, on := rs["info-description"]; on {
			if line, ok := commented(svc); !ok {
				add(r, finding{line: line, args: map[string]string{"spec": spec + " " + string(svc.Name())}})
			}
		}
		r, on := rs["operation-description"]
		if !on {
			continue
		}
		methods := svc.Methods()
		for j := range methods.Len() {
			m := methods.Get(j)
			if line, ok := commented(m); !ok {
				add(r, finding{line: line, args: map[string]string{"where": string(svc.Name()) + "." + string(m.Name())}})
			}
		}
	}
	return out
}
