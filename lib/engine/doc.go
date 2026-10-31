// SPDX-License-Identifier: Apache-2.0

// Package engine executes requests: it resolves the version and settings,
// runs API hooks, renders the request from templates and secrets, sends
// it, checks the response, and records and saves the result. Progress is
// reported as a stream of typed events (design §7.6), which the CLI and
// GUI render.
//
// Everything the engine records or emits is masked: values read from
// secrets never leave it in clear text.
package engine
