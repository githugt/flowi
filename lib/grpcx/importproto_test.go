// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReadProto_should_list_the_methods_and_the_local_files_it_needs(t *testing.T) {
	data, err := os.ReadFile("testdata/example.proto")
	if err != nil {
		t.Fatal(err)
	}

	got, err := ReadProto(context.Background(), "/anywhere/example.proto", data, "testdata")

	if err != nil || len(got.Methods) != 3 {
		t.Fatalf("methods %+v, err %v", got.Methods, err)
	}
	get := got.Methods[0]
	if get.Full() != "myexampleapp.ExampleService/GetExample" || get.ServerStream || get.Example != `{"id":""}` {
		t.Fatalf("get %+v", get)
	}
	if !got.Methods[1].ServerStream || !got.Methods[2].ClientStream {
		t.Fatalf("streams %+v", got.Methods)
	}
	if len(got.Files) != 2 || got.Files[0].Path != "example.proto" || got.Files[1].Path != "common/types.proto" {
		t.Fatalf("files %+v", got.Files)
	}
}

func TestReadProto_Given_a_missing_import_or_bad_syntax_Then_a_schema_error(t *testing.T) {
	_, missing := ReadProto(context.Background(), "a.proto", []byte(`syntax = "proto3"; import "nope.proto";`), "")
	_, bad := ReadProto(context.Background(), "b.proto", []byte(`this is not proto`), "")

	for _, err := range []error{missing, bad} {
		if se, ok := err.(*SchemaError); !ok || !se.Proto { //nolint:errorlint // the concrete type is the contract
			t.Fatalf("err %v", err)
		}
	}
}

// Before: an import such as "../secret.proto" was opened from disk during the
// compile, before the import's containment check ran at write time, so a
// hostile .proto could make Flow-I read files above its folder.
func TestReadProto_Given_an_import_outside_the_folder_Then_it_is_never_opened(t *testing.T) {
	tests := []string{"../secret.proto", "a/../../secret.proto", "/etc/hosts"}
	for _, imp := range tests {
		t.Run(imp, func(t *testing.T) {
			parent := t.TempDir()
			dir := filepath.Join(parent, "proto")
			if err := os.Mkdir(dir, 0o750); err != nil {
				t.Fatal(err)
			}
			secret := `syntax = "proto3"; message Secret {}`
			if err := os.WriteFile(filepath.Join(parent, "secret.proto"), []byte(secret), 0o600); err != nil {
				t.Fatal(err)
			}
			src := `syntax = "proto3"; import "` + imp + `";`

			_, err := ReadProto(context.Background(), "a.proto", []byte(src), dir)

			if se, ok := err.(*SchemaError); !ok || !se.Proto { //nolint:errorlint // the concrete type is the contract
				t.Fatalf("err %v, want a proto schema error", err)
			}
		})
	}
}
