package main

import (
	"runtime/debug"
	"strings"
)

// versionFromBuildInfo fills in version/commit/date from the Go build info
// embedded in every module-aware binary. This covers builds that bypass
// GoReleaser's ldflags — most importantly 'go install ...@version', which
// would otherwise report "dev (none) built unknown" for a real tagged
// release. Values already set via ldflags are left untouched.
func versionFromBuildInfo(bi *debug.BuildInfo, version, commit, date string) (string, string, string) {
	if bi == nil {
		return version, commit, date
	}

	if version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		version = strings.TrimPrefix(bi.Main.Version, "v")
	}

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "none" && len(s.Value) >= 7 {
				commit = s.Value[:7]
			}
		case "vcs.time":
			if date == "unknown" && s.Value != "" {
				date = s.Value
			}
		}
	}

	return version, commit, date
}
