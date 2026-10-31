// SPDX-License-Identifier: Apache-2.0

// Package telemetry is Flowi's opt-in usage reporting.
//
// It is off unless the user sets the telemetry setting to minimal or full,
// and nothing leaves the machine unless telemetry_endpoint is also set. What
// may be reported is fixed by Catalogue: every event and every value is
// listed there, either as a closed set of words or as a bucket, so a URL, a
// body, a name, an id or a path cannot be sent by mistake — Record and
// Payload.Validate refuse anything the catalogue does not name. There is no
// install id; payloads carry the day, not the time.
//
// `flowi telemetry show` and the GUI dialog print exactly what Sample and
// Payload would send, and docs/reference/telemetry.md is generated from
// Catalogue.
package telemetry
