// SPDX-License-Identifier: Apache-2.0

package version

import (
	"errors"
	"testing"
)

func TestResolve(t *testing.T) {
	ptr := func(t *testing.T, s string) *Label {
		l := mustParse(t, s)
		return &l
	}
	tests := []struct {
		name       string
		available  []string
		def        string
		pinned     string
		runPrefer  string
		flowPrefer string
		want       string
		wantSource Source
		wantErr    error
	}{
		{name: "pinned_present", available: []string{"1", "1.1", "2"}, pinned: "1.1", runPrefer: "2", want: "1.1", wantSource: SourcePinned},
		{name: "pinned_missing", available: []string{"1", "2"}, pinned: "3", wantErr: ErrPinnedMissing},
		{name: "run_beats_flow", available: []string{"1", "1.1", "2"}, runPrefer: "2", flowPrefer: "1.1", want: "2", wantSource: SourceRunPreference},
		{name: "absent_run_falls_to_flow", available: []string{"1", "1.1", "2"}, runPrefer: "3", flowPrefer: "1.1", want: "1.1", wantSource: SourceFlowPreference},
		{name: "default_used", available: []string{"1", "2"}, def: "1", want: "1", wantSource: SourceDefault},
		{name: "highest_when_no_default", available: []string{"1.10", "2", "1.2"}, want: "2", wantSource: SourceHighest},
		{name: "no_versions", wantErr: ErrNoVersions},
		{name: "default_missing", available: []string{"1"}, def: "9", wantErr: ErrDefaultMissing},
		{name: "preference_matches_equal_label", available: []string{"2.0"}, runPrefer: "2", want: "2.0", wantSource: SourceRunPreference},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var q Query
			for _, s := range tt.available {
				q.Available = append(q.Available, mustParse(t, s))
			}
			if tt.def != "" {
				q.Default = ptr(t, tt.def)
			}
			if tt.pinned != "" {
				q.Pinned = ptr(t, tt.pinned)
			}
			if tt.runPrefer != "" {
				q.RunPrefer = ptr(t, tt.runPrefer)
			}
			if tt.flowPrefer != "" {
				q.FlowPrefer = ptr(t, tt.flowPrefer)
			}

			got, source, err := Resolve(q)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("got err %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want || source != tt.wantSource {
				t.Fatalf("got (%s, %v), want (%s, %v)", got, source, tt.want, tt.wantSource)
			}
		})
	}
}
