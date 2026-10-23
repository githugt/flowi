// SPDX-License-Identifier: Apache-2.0

// Package shared is the state several `flowi serve` instances can share
// through a Valkey (or Redis) server: which schedule tick has been taken
// and how a run ended, so that GET /runs/<id> works on any instance.
//
// Everything in it is optional and fails open. The Valkey client is a
// small RESP client over the standard library; when the server cannot be
// reached it reports ErrUnavailable at once for a quiet period instead of
// waiting on every call, so a down cache never slows a run or a tick.
package shared
