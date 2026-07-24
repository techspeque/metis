package cli

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	versionCmd.SetOut(&out)
	t.Cleanup(func() { versionCmd.SetOut(nil) })

	if err := versionCmd.RunE(versionCmd, nil); err != nil {
		t.Fatalf("version command error: %v", err)
	}

	got := out.String()
	for _, want := range []string{"metis " + version, commit, date, runtime.GOOS + "/" + runtime.GOARCH} {
		if !strings.Contains(got, want) {
			t.Errorf("version output %q missing %q", got, want)
		}
	}
}
