// SPDX-License-Identifier: Apache-2.0

// Package template renders Flow-I's {{…}} templates. Each placeholder holds
// an expr-lang expression evaluated against a Scope of namespaces (env,
// vars, nodes, secret, request, response, …).
//
// Undefined references are errors unless guarded with ?? or ?., so typos
// surface instead of sending empty values. Values read from the secret
// namespace mark the result as tainted, which lets callers mask whole
// fields that were built from secrets.
//
// Before compiling, expressions are rewritten so the spec's notation works:
// generators ($first-name, $int(1, 9)) become calls, duration literals (2s,
// 800ms) become duration("…"), and comparisons with empty match null, "",
// and empty lists and maps.
package template
