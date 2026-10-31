// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtoFile is a .proto file with its import path.
type ProtoFile struct {
	Path string // as imports name it: "orders.proto", "common/types.proto"
	Data []byte
}

// ProtoImport is what a .proto file brings: the files a collection must hold
// for it to compile (the file and the local files it imports, transitively)
// and the methods of the services it declares.
type ProtoImport struct {
	Files   []ProtoFile
	Methods []Method
}

// ReadProto compiles the .proto file name (its text is data) with no protoc.
// Files it imports are read from dir, as imports name them; the well-known
// google/protobuf types are built in. dir may be empty when nothing is
// imported. Only the services declared by the file itself are returned.
func ReadProto(ctx context.Context, name string, data []byte, dir string) (ProtoImport, error) {
	name = filepath.ToSlash(filepath.Base(name))
	if name == "" || name == "." {
		name = "input.proto"
	}
	local := &protocompile.SourceResolver{Accessor: func(p string) (io.ReadCloser, error) {
		if p == name {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
		if dir == "" || !localImportPath(p) {
			return nil, os.ErrNotExist
		}
		return os.Open(filepath.Join(dir, filepath.FromSlash(p))) //nolint:gosec // an import named by the user's own .proto file, read from their folder
	}}
	compiled, err := (&protocompile.Compiler{Resolver: protocompile.WithStandardImports(local)}).Compile(ctx, name)
	if err != nil {
		return ProtoImport{}, &SchemaError{Detail: "compiling " + name + ": " + err.Error(), Proto: true}
	}
	out := ProtoImport{Files: []ProtoFile{{Path: name, Data: data}}}
	main := compiled[0]
	services := main.Services()
	for i := range services.Len() {
		ms := services.Get(i).Methods()
		for j := range ms.Len() {
			out.Methods = append(out.Methods, methodOf(ms.Get(j)))
		}
	}
	seen := map[string]bool{name: true}
	if err := collectImports(main, dir, seen, &out); err != nil {
		return ProtoImport{}, err
	}
	return out, nil
}

// collectImports adds the local files fd imports, depth first.
func collectImports(fd protoreflect.FileDescriptor, dir string, seen map[string]bool, out *ProtoImport) error {
	imports := fd.Imports()
	for i := range imports.Len() {
		imp := imports.Get(i).FileDescriptor
		path := imp.Path()
		if seen[path] || strings.HasPrefix(path, "google/protobuf/") {
			continue
		}
		seen[path] = true
		if !localImportPath(path) {
			return &SchemaError{Detail: fmt.Sprintf("the imported file %s is outside the .proto file's folder", path), Proto: true}
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path))) //nolint:gosec // see ReadProto
		if err != nil {
			return &SchemaError{Detail: fmt.Sprintf("reading the imported file %s: %v", path, err), Proto: true}
		}
		out.Files = append(out.Files, ProtoFile{Path: path, Data: data})
		if err := collectImports(imp, dir, seen, out); err != nil {
			return err
		}
	}
	slices.SortFunc(out.Files[1:], func(a, b ProtoFile) int { return strings.Compare(a.Path, b.Path) })
	return nil
}

// localImportPath reports whether an import path stays inside the .proto
// file's folder: relative, with no ".." part. A .proto file is untrusted
// text, and its imports are read from disk before anything is written, so
// "../../secret" or "/etc/x" must not be opened.
func localImportPath(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) || filepath.IsAbs(p) {
		return false
	}
	return !slices.Contains(strings.Split(p, "/"), "..")
}
