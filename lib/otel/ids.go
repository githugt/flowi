// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Trace and span ids are derived from the run id and the node path, so the
// engine can name a span in a traceparent header before the span is
// exported, and the exporter needs no shared state to agree with it.

const (
	traceIDBytes = 16
	spanIDBytes  = 8
)

func traceID(runID string) string { return derive("trace", runID, "", traceIDBytes) }

func spanID(runID, kind, key string) string { return derive("span/"+kind, runID, key, spanIDBytes) }

// rootSpanID is the id of a run's top span.
func rootSpanID(runID string) string { return spanID(runID, "root", "") }

// nodeSpanID is the id of the span of the node instance at path.
func nodeSpanID(runID, path string) string { return spanID(runID, "node", path) }

// derive hashes the parts to n bytes of lower-case hex. An all-zero id is
// invalid in W3C Trace Context; a hash of that kind is not a case worth a
// branch, but the first byte is forced non-zero to make it impossible.
func derive(domain, runID, key string, n int) string {
	sum := sha256.Sum256([]byte(domain + "\x00" + runID + "\x00" + key))
	b := sum[:n]
	if b[0] == 0 {
		b[0] = 1
	}
	return hex.EncodeToString(b)
}

// traceparent formats a W3C traceparent header (version 00, sampled).
func traceparent(trace, span string) string { return "00-" + trace + "-" + span + "-01" }

// TraceID is the trace id of a run, as the exporter and the traceparent header write it (32 lower-case hex).
func TraceID(runID string) string { return traceID(runID) }

// TraceURL puts a run's trace id into a tracing tool's address template, where {trace_id} stands for it. It is ""
// for an empty template, an empty run id, or a template with no {trace_id} (which would open the same page for
// every run).
func TraceURL(template, runID string) string {
	if template == "" || runID == "" || !strings.Contains(template, "{trace_id}") {
		return ""
	}
	return strings.ReplaceAll(template, "{trace_id}", traceID(runID))
}
