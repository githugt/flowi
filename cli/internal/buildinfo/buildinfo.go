// SPDX-License-Identifier: AGPL-3.0-or-later

// Package buildinfo reports the version of the running flowi binary.
package buildinfo

import "runtime/debug"

const (
	devVersion        = "dev"
	develMarker       = "(devel)"
	shortRevisionSize = 12
)

// stamped is the release version, given to the linker by the release build
// (-ldflags "-X .../buildinfo.stamped=v1.2.3"). Go cannot supply it: a module in
// a subfolder of the repository is not stamped from its tag, and release builds
// use -buildvcs=false. It has to be a package variable for the linker to set it;
// nothing else writes it.
var stamped string //nolint:gochecknoglobals // set by the linker, read-only at run time

// Version returns the release version for release builds, the module version
// for other tagged builds, "dev+<revision>" for local builds with VCS metadata,
// and "dev" otherwise.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	return versionFrom(stamped, info, ok)
}

func versionFrom(stamped string, info *debug.BuildInfo, ok bool) string {
	if stamped != "" {
		return stamped
	}
	if !ok || info == nil {
		return devVersion
	}
	if v := info.Main.Version; v != "" && v != develMarker {
		return v
	}
	revision := settingValue(info, "vcs.revision")
	if revision == "" {
		return devVersion
	}
	return devVersion + "+" + revision[:min(len(revision), shortRevisionSize)]
}

func settingValue(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
