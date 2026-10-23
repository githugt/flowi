// SPDX-License-Identifier: Apache-2.0

// Package examples holds the example collections, as documentation and as
// fixtures for the CLI and GUI tests.
package examples

import (
	"embed"
	"io/fs"
)

//go:embed all:user-creation
var collections embed.FS

// UserCreation is the reference collection: a user signs up, verifies and
// completes onboarding, against the demo API in examples/demoserver. Its
// local environment points at http://localhost:8080.
func UserCreation() fs.FS {
	sub, err := fs.Sub(collections, "user-creation")
	if err != nil {
		panic(err) // the directory is embedded above; a failure is a build defect
	}
	return sub
}
