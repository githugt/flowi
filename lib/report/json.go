// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
)

// WriteJSON writes r as indented JSON. Durations are written as
// milliseconds (duration_ms, grace_ms).
func WriteJSON(w io.Writer, r Run) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("writing JSON report: %w", err)
	}
	return nil
}

// The MarshalJSON methods add duration_ms next to the other fields; the
// time.Duration fields themselves are hidden (json:"-") because
// nanoseconds are unreadable in a report.

// MarshalJSON adds duration_ms.
func (r Run) MarshalJSON() ([]byte, error) {
	type plain Run
	return json.Marshal(struct {
		plain
		DurationMS int64 `json:"duration_ms"`
	}{plain(r), r.Duration.Milliseconds()})
}

// MarshalJSON adds duration_ms.
func (n Node) MarshalJSON() ([]byte, error) {
	type plain Node
	return json.Marshal(struct {
		plain
		DurationMS int64 `json:"duration_ms"`
	}{plain(n), n.Duration.Milliseconds()})
}

// MarshalJSON adds duration_ms.
func (h Hook) MarshalJSON() ([]byte, error) {
	type plain Hook
	return json.Marshal(struct {
		plain
		DurationMS int64 `json:"duration_ms"`
	}{plain(h), h.Duration.Milliseconds()})
}

// MarshalJSON adds duration_ms.
func (a Attempt) MarshalJSON() ([]byte, error) {
	type plain Attempt
	return json.Marshal(struct {
		plain
		DurationMS int64 `json:"duration_ms"`
	}{plain(a), a.Duration.Milliseconds()})
}

// MarshalJSON adds grace_ms.
func (a Abort) MarshalJSON() ([]byte, error) {
	type plain Abort
	return json.Marshal(struct {
		plain
		GraceMS int64 `json:"grace_ms"`
	}{plain(a), a.Grace.Milliseconds()})
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }
