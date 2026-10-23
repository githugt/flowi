// SPDX-License-Identifier: Apache-2.0

// Package otel turns the engine's event stream into OpenTelemetry traces:
// one trace per run, a span per node and per hook step, and the request
// attempts as span events. Traces leave as OTLP over HTTP with a JSON body,
// which every collector accepts; no SDK is involved.
//
// It is off unless Config.Endpoint is set. The Exporter is wired in through
// engine.Deps.Observe and engine.Deps.Traceparent and never affects a run:
// Observe does not block, and export failures are logged and reported by
// Close, nothing more.
package otel
