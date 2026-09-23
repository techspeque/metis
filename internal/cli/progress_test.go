package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runCaptured(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	err := rootCmd.Execute()
	return out.String(), err
}

// TestProgressViews: bare 'progress' is the stage view; stats, stage and
// phase each pick their own breakdown; anything else is rejected.
func TestProgressViews(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "")

	cases := map[string]string{
		"":        "No slices have a stage.",
		"stage":   "No slices have a stage.",
		"stages":  "No slices have a stage.",
		"stats":   "Pending:   1",
		"summary": "Pending:   1",
		"phase":   "By Phase:",
		"phases":  "By Phase:",
	}
	for view, want := range cases {
		args := []string{"progress"}
		if view != "" {
			args = append(args, view)
		}
		out, err := runCaptured(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.Contains(out, "Overall: 0/1 done") || !strings.Contains(out, want) {
			t.Errorf("%v: missing %q in:\n%s", args, want, out)
		}
	}

	if _, err := runCaptured(t, "progress", "bogus"); err == nil {
		t.Error("progress bogus: want error")
	}
}

func TestProgressViewsJSON(t *testing.T) {
	makeProjectWithLedger(t)
	setOutputFlag(t, "json")
	for _, view := range []string{"stats", "stage", "phase"} {
		var d struct {
			Total   int `json:"total"`
			Pending int `json:"pending"`
		}
		sub, _, err := rootCmd.Find([]string{"progress", view})
		if err != nil {
			t.Fatal(err)
		}
		captureJSON(t, sub, nil, &d)
		if d.Total != 1 || d.Pending != 1 {
			t.Errorf("progress %s JSON = %+v", view, d)
		}
	}
}
