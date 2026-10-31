// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"encoding/json"

	"github.com/githugt/flowi-lib/telemetry"
)

// Telemetry is the dialog where the user chooses what usage reporting may
// send: the plain-language explanation, a preview of the report for the
// level under consideration, and the choice itself.
type Telemetry struct {
	b        Backend
	w        *Worker
	client   telemetry.Client
	level    string
	problem  string
	onChange []func()
}

// NewTelemetry returns the dialog's view-model; call Load. client describes
// this program in previews.
func NewTelemetry(b Backend, w *Worker, client telemetry.Client) *Telemetry {
	return &Telemetry{b: b, w: w, client: client, level: "off"}
}

// OnChange registers f to run after the level or the problem changes.
func (t *Telemetry) OnChange(f func()) { t.onChange = append(t.onChange, f) }

func (t *Telemetry) changed() {
	for _, f := range t.onChange {
		f()
	}
}

// Explanation is the plain-language statement of what telemetry is.
func (t *Telemetry) Explanation() string { return telemetry.Explanation() }

// Level is the current choice: off, minimal or full.
func (t *Telemetry) Level() string { return t.level }

// Problem is why the last Choose was refused, or "".
func (t *Telemetry) Problem() string { return t.problem }

// Load reads the current level in the background.
func (t *Telemetry) Load() {
	Run(t.w, func() string { return levelSetting(t.b) }, func(level string) {
		t.level = level
		t.changed()
	})
}

func levelSetting(b Backend) string {
	v, _ := b.Settings("").Get("telemetry")
	if s, ok := v.Value.(string); ok && s != "" {
		return s
	}
	return "off"
}

// Preview is the report that level would send, with example values, as
// indented JSON.
func (t *Telemetry) Preview(level string) string {
	data, err := json.MarshalIndent(telemetry.Sample(telemetry.ParseLevel(level), t.client), "", "  ")
	if err != nil {
		return err.Error() // a sample of plain strings cannot fail to encode
	}
	return string(data)
}

// Choose stores level in the user settings in the background.
func (t *Telemetry) Choose(level string) {
	Run(t.w, func() error { return t.b.SetUserSetting("telemetry", level) }, func(err error) {
		t.problem = ""
		if err != nil {
			t.problem = ErrorText(err)
		} else {
			t.level = level
		}
		t.changed()
	})
}
