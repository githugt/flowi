# flowi: run the apps and checks across lib, cli and app. Run `just` to list recipes.
#
# Each folder has its own Justfile with more recipes; these delegate to them.

set shell := ["bash", "-euo", "pipefail", "-c"]
set positional-arguments

repos := "lib cli app"

# List recipes.
default:
    @just --list

# Start the GUI (Wails + Svelte).
run *args:
    just --justfile app/Justfile run "$@"

# Run a CLI command in the cli sandbox (never touches your real settings), e.g. `just run-cli ls requests`.
run-cli *args:
    just --justfile cli/Justfile cli "$@"

# Serve the demo API the example collection talks to (127.0.0.1:8080, password "demo").
demo-server *args:
    just --justfile lib/Justfile demo-server "$@"

# Reset the CLI sandbox to just the example collection.
sandbox:
    just --justfile cli/Justfile sandbox

# Walk through the main CLI commands on a fresh sandbox.
demo:
    just --justfile cli/Justfile demo

# Run both example flows against the demo API (live progress tree), writing JUnit and Markdown reports.
demo-run:
    just --justfile cli/Justfile demo-run

# Run every module's full test suite.
test-all:
    for r in {{repos}}; do echo "== $r"; just --justfile $r/Justfile test-all; done

# Format, tidy and verify modules in every module.
clean:
    for r in {{repos}}; do echo "== $r"; just --justfile $r/Justfile clean; done

# Lint every module.
lint:
    for r in {{repos}}; do echo "== $r"; just --justfile $r/Justfile lint; done

# clean, lint and test-all across every module.
check: clean lint test-all
