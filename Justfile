# Flow-I web-view GUI (Wails 3 + Svelte). Run `just` to list recipes.

set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

# Go's cgo passes -lobjc twice on macOS, and Xcode 15+'s linker warns about it
# (golang/go#67799). The warning is harmless; this silences it.
ldflags := if os() == "macos" { "-ldflags=-extldflags=-Wl,-no_warn_duplicate_libraries" } else { "" }

# Build the frontend, then start the app.
run *args: frontend
    go run {{ldflags}} . "$@"

# Build the app into ./bin/flowi (the front end first: the binary embeds it).
build: frontend
    go build {{ldflags}} -o bin/flowi .

# Install the frontend dependencies when missing, and build it (the app embeds frontend/dist).
frontend:
    cd frontend && { [ -d node_modules ] || npm ci; } && npm run build

# Regenerate the TypeScript bindings after changing a service's Go methods.
bindings:
    wails3 generate bindings -clean=true

# Type-check the Svelte code.
check-frontend:
    cd frontend && npm run check

# Format code, tidy go.mod and verify downloaded modules.
clean:
    go fmt ./...
    go mod tidy
    go mod verify

# Lint with golangci-lint (v2).
lint:
    golangci-lint run ./...

# Run every test with the race detector, and type-check the frontend.
test-all: frontend check-frontend test-frontend
    go test -race ./...

# Run the frontend's unit tests.
test-frontend:
    cd frontend && npm test

# Everything CI checks: clean, lint, test-all.
check: clean lint test-all
