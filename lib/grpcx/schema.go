// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	// The well-known types are registered by importing them: a schema that
	// imports google/protobuf/timestamp.proto resolves it from here, and
	// protojson converts them.
	_ "google.golang.org/protobuf/types/known/anypb"
	_ "google.golang.org/protobuf/types/known/durationpb"
	_ "google.golang.org/protobuf/types/known/emptypb"
	_ "google.golang.org/protobuf/types/known/fieldmaskpb"
	_ "google.golang.org/protobuf/types/known/structpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
	_ "google.golang.org/protobuf/types/known/wrapperspb"
)

const noReflection = "the server does not offer reflection: add the service's .proto files to the request"

// Methods lists the methods of the schema a call would use: its .proto files,
// or every service the server's reflection lists.
func Methods(ctx context.Context, c Call) ([]Method, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	if len(c.Protos) > 0 {
		files, err := compileProtos(ctx, c)
		if err != nil {
			return nil, err
		}
		return methodsIn(files), nil
	}
	conn, err := dial(c)
	if err != nil {
		return nil, err
	}
	defer conn.Close() //nolint:errcheck // nothing useful to do if closing fails
	files, err := reflectAll(ctx, conn)
	if err != nil {
		return nil, err
	}
	return methodsIn(files), nil
}

// methodsIn lists every method of the services in files (not the well-known
// types' own).
func methodsIn(files *protoregistry.Files) []Method {
	var out []Method
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			ms := services.Get(i).Methods()
			for j := 0; j < ms.Len(); j++ {
				out = append(out, methodOf(ms.Get(j)))
			}
		}
		return true
	})
	return out
}

func methodOf(m protoreflect.MethodDescriptor) Method {
	return Method{
		Service: string(m.Parent().FullName()), Name: string(m.Name()),
		ClientStream: m.IsStreamingClient(), ServerStream: m.IsStreamingServer(),
		Input: string(m.Input().FullName()), Output: string(m.Output().FullName()), Example: exampleMessage(m.Input()),
	}
}

// resolveMethod finds the descriptor of the call's method: in its .proto
// files, or by asking the server.
func resolveMethod(ctx context.Context, conn *grpc.ClientConn, c Call) (protoreflect.MethodDescriptor, error) {
	service, name, ok := splitMethod(c.Method)
	if !ok {
		return nil, &SchemaError{Detail: fmt.Sprintf("method %q is written package.Service/Method", c.Method)}
	}
	var files *protoregistry.Files
	var err error
	if len(c.Protos) > 0 {
		files, err = compileProtos(ctx, c)
	} else {
		files, err = reflectService(ctx, conn, service)
	}
	if err != nil {
		return nil, err
	}
	d, err := files.FindDescriptorByName(protoreflect.FullName(service))
	sd, isService := d.(protoreflect.ServiceDescriptor)
	if err != nil || !isService {
		return nil, &SchemaError{Detail: fmt.Sprintf("service %s is not in the schema", service)}
	}
	md := sd.Methods().ByName(protoreflect.Name(name))
	if md == nil {
		return nil, &SchemaError{Detail: fmt.Sprintf("method %s is not in service %s", name, service)}
	}
	return md, nil
}

func splitMethod(m string) (service, method string, ok bool) {
	service, method, found := strings.Cut(strings.TrimPrefix(m, "/"), "/")
	return service, method, found && service != "" && method != "" && !strings.Contains(method, "/")
}

// compileProtos compiles the call's .proto files against its import roots
// (and the collection root itself) with no protoc, and registers them with
// everything they import.
func compileProtos(ctx context.Context, c Call) (*protoregistry.Files, error) {
	roots := make([]string, 0, len(c.Imports)+1)
	for _, imp := range c.Imports {
		roots = append(roots, filepath.Join(c.Root, filepath.FromSlash(imp)))
	}
	roots = append(roots, c.Root)
	names := make([]string, 0, len(c.Protos))
	for _, p := range c.Protos {
		abs := filepath.Join(c.Root, filepath.FromSlash(p))
		names = append(names, nameUnderRoots(abs, roots))
	}
	compiler := protocompile.Compiler{Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{ImportPaths: roots})}
	compiled, err := compiler.Compile(ctx, names...)
	if err != nil {
		return nil, &SchemaError{Detail: "compiling the .proto files: " + err.Error(), Proto: true}
	}
	files := &protoregistry.Files{}
	for _, f := range compiled {
		if err := registerWithImports(files, f); err != nil {
			return nil, &SchemaError{Detail: "registering the .proto files: " + err.Error(), Proto: true}
		}
	}
	return files, nil
}

// nameUnderRoots is the name of the file abs as an import path: relative to
// the first root that holds it.
func nameUnderRoots(abs string, roots []string) string {
	for _, r := range roots {
		if rel, err := filepath.Rel(r, abs); err == nil && filepath.IsLocal(rel) {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(abs)
}

// registerWithImports adds fd and, first, everything it imports.
func registerWithImports(files *protoregistry.Files, fd protoreflect.FileDescriptor) error {
	if _, err := files.FindFileByPath(fd.Path()); err == nil {
		return nil
	}
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		if err := registerWithImports(files, imports.Get(i).FileDescriptor); err != nil {
			return err
		}
	}
	return files.RegisterFile(fd)
}

// buildFiles turns serialized FileDescriptorProtos into registered files,
// asking fetch for imports that were not sent (and the well-known ones come
// from the global registry). initial is what the server sent first.
func buildFiles(initial [][]byte, fetch func(name string) ([][]byte, error)) (*protoregistry.Files, error) {
	protos := map[string]*descriptorpb.FileDescriptorProto{}
	var queue []*descriptorpb.FileDescriptorProto
	add := func(raw [][]byte) error {
		for _, b := range raw {
			fdp := &descriptorpb.FileDescriptorProto{}
			if err := unmarshalFile(b, fdp); err != nil {
				return err
			}
			if _, seen := protos[fdp.GetName()]; !seen {
				protos[fdp.GetName()] = fdp
				queue = append(queue, fdp)
			}
		}
		return nil
	}
	if err := add(initial); err != nil {
		return nil, err
	}
	for len(queue) > 0 {
		next := queue[0]
		queue = queue[1:]
		for _, dep := range next.GetDependency() {
			if _, have := protos[dep]; have {
				continue
			}
			if _, err := protoregistry.GlobalFiles.FindFileByPath(dep); err == nil {
				continue
			}
			raw, err := fetch(dep)
			if err != nil {
				return nil, err
			}
			if err := add(raw); err != nil {
				return nil, err
			}
		}
	}
	files := &protoregistry.Files{}
	done := map[string]bool{}
	var register func(name string) error
	register = func(name string) error {
		if done[name] {
			return nil
		}
		fdp, ok := protos[name]
		if !ok {
			return nil // a global (well-known) file
		}
		done[name] = true
		for _, dep := range fdp.GetDependency() {
			if err := register(dep); err != nil {
				return err
			}
		}
		fd, err := protodesc.NewFile(fdp, combinedResolver{local: files})
		if err != nil {
			return fmt.Errorf("building %s: %w", name, err)
		}
		return files.RegisterFile(fd)
	}
	for name := range protos {
		if err := register(name); err != nil {
			return nil, &SchemaError{Detail: "the server's schema is not usable: " + err.Error()}
		}
	}
	return files, nil
}

// combinedResolver finds files among those built so far, then among the
// well-known ones every program has.
type combinedResolver struct{ local *protoregistry.Files }

func (r combinedResolver) FindFileByPath(p string) (protoreflect.FileDescriptor, error) {
	if fd, err := r.local.FindFileByPath(p); err == nil {
		return fd, nil
	}
	return protoregistry.GlobalFiles.FindFileByPath(p)
}

func (r combinedResolver) FindDescriptorByName(n protoreflect.FullName) (protoreflect.Descriptor, error) {
	if d, err := r.local.FindDescriptorByName(n); err == nil {
		return d, nil
	}
	return protoregistry.GlobalFiles.FindDescriptorByName(n)
}

var errNoServices = errors.New("the server lists no services")
