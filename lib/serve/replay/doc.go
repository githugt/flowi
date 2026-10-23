// SPDX-License-Identifier: Apache-2.0

// Package replay is the replayer: an http.Handler that answers requests
// with responses the user already has, the pinned examples next to a
// request file and the saved responses in the workspace data dir, instead
// of calling the real API.
//
// A request matches a saved request by method and URL path (see route.go);
// the response is the request's default one unless X-Flowi-Status or
// X-Flowi-Example pick another (see picks.go). The handler reads files on
// each call, so a pinned example shows up at once; Refresh rebuilds the
// route table after requests are added or changed.
//
// The package keeps no global state. The time source for simulated latency
// is injected (Options.Sleep) so tests do not wait.
package replay
