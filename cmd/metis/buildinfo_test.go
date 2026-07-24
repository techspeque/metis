package main

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfo(t *testing.T) {
	tests := []struct {
		name                  string
		bi                    *debug.BuildInfo
		version, commit, date string
		wantV, wantC, wantD   string
	}{
		{
			name:    "nil build info leaves defaults",
			bi:      nil,
			version: "dev", commit: "none", date: "unknown",
			wantV: "dev", wantC: "none", wantD: "unknown",
		},
		{
			name: "go install build fills everything",
			bi: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.0.2"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "62f1ed9abcdef0123456789"},
					{Key: "vcs.time", Value: "2026-07-24T18:15:01Z"},
				},
			},
			version: "dev", commit: "none", date: "unknown",
			wantV: "0.0.2", wantC: "62f1ed9", wantD: "2026-07-24T18:15:01Z",
		},
		{
			name: "ldflags values are never overridden",
			bi: &debug.BuildInfo{
				Main: debug.Module{Version: "v9.9.9"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "ffffffffffffffff"},
					{Key: "vcs.time", Value: "2020-01-01T00:00:00Z"},
				},
			},
			version: "0.0.2", commit: "62f1ed9", date: "2026-07-24T18:15:01Z",
			wantV: "0.0.2", wantC: "62f1ed9", wantD: "2026-07-24T18:15:01Z",
		},
		{
			name:    "(devel) source build keeps dev version",
			bi:      &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			version: "dev", commit: "none", date: "unknown",
			wantV: "dev", wantC: "none", wantD: "unknown",
		},
		{
			name:    "short vcs revision is ignored",
			bi:      &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "abc"}}},
			version: "dev", commit: "none", date: "unknown",
			wantV: "dev", wantC: "none", wantD: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, c, d := versionFromBuildInfo(tt.bi, tt.version, tt.commit, tt.date)
			if v != tt.wantV || c != tt.wantC || d != tt.wantD {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", v, c, d, tt.wantV, tt.wantC, tt.wantD)
			}
		})
	}
}
