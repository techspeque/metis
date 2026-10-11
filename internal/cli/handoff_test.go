package cli

import (
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func resetFlags(t *testing.T, flags *pflag.FlagSet) {
	t.Helper()
	reset := func() {
		flags.VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
	reset()
	t.Cleanup(reset)
}

func TestHandoffJSON(t *testing.T) {
	makeGitProjectWithLedger(t)
	setOutputFlag(t, "json")
	resetFlags(t, handoffCmd.Flags())

	var out handoffResult
	captureJSON(t, handoffCmd, nil, &out)
	if out.Slice != "feat-0001" || out.Role != "Coder" || out.Template != "code" || out.AgentSlug != "claude-code/opus" {
		t.Errorf("handoff = %+v", out)
	}
	if !strings.Contains(out.Prompt, "metis brief feat-0001 --write") {
		t.Errorf("code prompt should ask for the brief:\n%s", out.Prompt)
	}

	_ = handoffCmd.Flags().Set("template", "review")
	captureJSON(t, handoffCmd, []string{"feat-0001"}, &out)
	if out.Role != "Reviewer" || out.AgentSlug != "opencode/opus" || out.Template != "review" {
		t.Errorf("forced review = %+v", out)
	}
}
