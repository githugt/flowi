// SPDX-License-Identifier: Apache-2.0

// Package analysis answers "which flows and hooks break if this API
// changes?" (design §9). For each request version it finds the consumers
// (flow request nodes, post-hooks, and the version's own extract and
// assert), what each one reads from the response and which inputs it
// fills, and checks those reads against the best contract available:
// pinned examples, else the last saved response.
//
// The package does no logging and reads files only through Contracts.
package analysis
