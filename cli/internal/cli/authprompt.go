// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/githugt/flowi-lib/engine"
)

// browserWait bounds how long starting the browser may take.
const browserWait = 10 * time.Second

// watchPrompts passes events on and, for a request that waits for the person
// to sign in, tries to open the page in their browser. The renderers print
// the prompt themselves, so the address is always on screen when no browser
// opens (a server, a CI job, a terminal over ssh).
//
// Goroutine ownership: one goroutine, started here, that ends when in closes.
// The caller must drain the returned channel as it would in.
func watchPrompts(in <-chan engine.Event, open func(string) error) <-chan engine.Event {
	out := make(chan engine.Event)
	go func() {
		defer close(out)
		for ev := range in {
			if p, ok := ev.(engine.AuthPrompted); ok && p.Kind == "open" {
				_ = open(p.URL) // best effort: the prompt line names the address for anyone whose browser did not open
			}
			out <- ev
		}
	}()
	return out
}

// openBrowser asks the system to open a web address. It refuses anything that
// is not http or https, so a provider's response cannot make it run something else.
func openBrowser(address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("not a web address: %q", address)
	}
	ctx, cancel := context.WithTimeout(context.Background(), browserWait)
	defer cancel()
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", address) //nolint:gosec // address is an http(s) URL checked above, passed as one argument, never through a shell
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", address) //nolint:gosec // as above
	default:
		cmd = exec.CommandContext(ctx, "xdg-open", address) //nolint:gosec // as above
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opening a browser: %w", err)
	}
	return nil
}
