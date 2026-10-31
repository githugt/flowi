// SPDX-License-Identifier: AGPL-3.0-or-later

package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestVersionFrom(t *testing.T) {
	tests := []struct {
		name    string
		stamped string
		info    *debug.BuildInfo
		ok      bool
		want    string
	}{
		{name: "should_return_dev_when_build_info_missing", info: nil, ok: false, want: "dev"},
		{
			name: "should_return_dev_for_devel_without_revision",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			ok:   true,
			want: "dev",
		},
		{
			name: "should_return_module_version_when_tagged",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.1.0"}},
			ok:   true,
			want: "v0.1.0",
		},
		{
			// A release build is made with -buildvcs=false and from a module in a subfolder, so Go stamps no
			// version of its own: the one the release workflow gives the linker is the only one there is.
			name:    "should_return_stamped_version_over_what_go_stamped",
			stamped: "v0.4.0-alpha.1",
			info:    &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			ok:      true,
			want:    "v0.4.0-alpha.1",
		},
		{
			name: "should_append_short_revision_for_devel_build",
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "0123456789abcdef0123"}},
			},
			ok:   true,
			want: "dev+0123456789ab",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := versionFrom(tt.stamped, tt.info, tt.ok)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
