// SPDX-License-Identifier: Apache-2.0

// Package announce fetches Flowi's announcements: a short list of notices
// from the project, shown at startup and by `flowi news`.
//
// The feed is a JSON document {"payload": base64, "signature": base64}
// whose signature is an Ed25519 signature of the decoded payload. Only a
// feed signed by a configured key is shown, whatever served it. The
// fetch is quiet by design: it has a short timeout, caches the last good
// feed, never follows redirects and, when anything goes wrong, still
// returns what the cache holds along with the reason.
//
// Nothing is fetched unless a feed URL and at least one trusted key are
// configured; there is no built-in address.
package announce
