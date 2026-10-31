// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"encoding/json"
	"strconv"
	"time"
)

// scopeName identifies this instrumentation in the exported scope.
const scopeName = "github.com/githugt/flowi/lib/otel"

// The types below are the OTLP/JSON encoding of an ExportTraceServiceRequest
// (opentelemetry-proto, JSON mapping: ids as hex strings, 64-bit integers
// as decimal strings).
type (
	otlpRequest struct {
		ResourceSpans []otlpResourceSpans `json:"resourceSpans"`
	}
	otlpResourceSpans struct {
		Resource   otlpResource     `json:"resource"`
		ScopeSpans []otlpScopeSpans `json:"scopeSpans"`
	}
	otlpResource struct {
		Attributes []otlpKeyValue `json:"attributes"`
	}
	otlpScopeSpans struct {
		Scope otlpScope  `json:"scope"`
		Spans []otlpSpan `json:"spans"`
	}
	otlpScope struct {
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
	}
	otlpSpan struct {
		TraceID           string         `json:"traceId"`
		SpanID            string         `json:"spanId"`
		ParentSpanID      string         `json:"parentSpanId,omitempty"`
		Name              string         `json:"name"`
		Kind              int            `json:"kind"`
		StartTimeUnixNano string         `json:"startTimeUnixNano"`
		EndTimeUnixNano   string         `json:"endTimeUnixNano"`
		Attributes        []otlpKeyValue `json:"attributes,omitempty"`
		Events            []otlpEvent    `json:"events,omitempty"`
		Status            otlpStatus     `json:"status"`
	}
	otlpEvent struct {
		TimeUnixNano string         `json:"timeUnixNano"`
		Name         string         `json:"name"`
		Attributes   []otlpKeyValue `json:"attributes,omitempty"`
	}
	otlpStatus struct {
		Code    int    `json:"code"`
		Message string `json:"message,omitempty"`
	}
	otlpKeyValue struct {
		Key   string    `json:"key"`
		Value otlpValue `json:"value"`
	}
	otlpValue struct {
		String *string `json:"stringValue,omitempty"`
		Int    *string `json:"intValue,omitempty"`
		Bool   *bool   `json:"boolValue,omitempty"`
	}
)

func nanos(t time.Time) string { return strconv.FormatInt(t.UnixNano(), 10) }

func encodeAttrs(attrs []attr) []otlpKeyValue {
	out := make([]otlpKeyValue, 0, len(attrs))
	for _, a := range attrs {
		kv := otlpKeyValue{Key: a.key}
		switch v := a.val.(type) {
		case string:
			if v == "" {
				continue // an empty attribute says nothing
			}
			kv.Value.String = &v
		case int64:
			s := strconv.FormatInt(v, 10)
			kv.Value.Int = &s
		case bool:
			kv.Value.Bool = &v
		default:
			continue
		}
		out = append(out, kv)
	}
	return out
}

func encodeSpan(traceID string, s *spanRec) otlpSpan {
	o := otlpSpan{
		TraceID: traceID, SpanID: s.id, ParentSpanID: s.parent, Name: s.name, Kind: s.kind,
		StartTimeUnixNano: nanos(s.start), EndTimeUnixNano: nanos(s.end),
		Attributes: encodeAttrs(s.attrs), Status: otlpStatus{Code: s.status, Message: s.message},
	}
	for _, e := range s.events {
		o.Events = append(o.Events, otlpEvent{TimeUnixNano: nanos(e.at), Name: e.name, Attributes: encodeAttrs(e.attrs)})
	}
	return o
}

// encodeTrace is the request body for one finished trace.
func encodeTrace(t *trace, service, version string) ([]byte, error) {
	spans := make([]otlpSpan, 0, len(t.spans))
	for _, s := range t.spans {
		spans = append(spans, encodeSpan(t.id, s))
	}
	req := otlpRequest{ResourceSpans: []otlpResourceSpans{{
		Resource: otlpResource{Attributes: encodeAttrs([]attr{{"service.name", service}, {"service.version", version}})},
		ScopeSpans: []otlpScopeSpans{{
			Scope: otlpScope{Name: scopeName},
			Spans: spans,
		}},
	}}}
	return json.Marshal(req)
}
