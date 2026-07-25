package git

import (
	"testing"

	"github.com/techspeque/metis/internal/config"
	"github.com/techspeque/metis/internal/slice"
)

func TestFormatCommitMessage(t *testing.T) {
	cfg := &config.Config{
		Commits: config.CommitsConfig{
			Format: "{prefix}({slice_id}): {message}",
		},
	}

	tests := []struct {
		sliceID string
		prefix  string
		message string
		want    string
	}{
		{"feat-0001", "feat", "add webhook handler", "feat(feat-0001): add webhook handler"},
		{"fix-0003", "fix", "close auth bypass", "fix(fix-0003): close auth bypass"},
		{"phase-2-ws-2.3", "test", "add integration tests", "test(phase-2-ws-2.3): add integration tests"},
	}

	for _, tt := range tests {
		got := FormatCommitMessage(cfg, tt.sliceID, tt.prefix, tt.message)
		if got != tt.want {
			t.Errorf("FormatCommitMessage(%q, %q, %q) = %q, want %q",
				tt.sliceID, tt.prefix, tt.message, got, tt.want)
		}
	}
}

func TestInferPrefix(t *testing.T) {
	tests := []struct {
		wt   slice.WorkType
		want string
	}{
		{slice.TypeFeat, "feat"},
		{slice.TypeFix, "fix"},
		{slice.TypeRefactor, "refactor"},
		{slice.TypeDebt, "refactor"},
		{slice.TypeRemove, "refactor"},
		{slice.TypeChore, "chore"},
		{slice.TypeSecurity, "fix"},
		{slice.TypeGate, "docs"},
		{slice.TypeRecon, "docs"},
	}
	for _, tt := range tests {
		if got := InferPrefix(tt.wt); got != tt.want {
			t.Errorf("InferPrefix(%q) = %q, want %q", tt.wt, got, tt.want)
		}
	}
}

func TestValidatePrefix(t *testing.T) {
	cfg := &config.Config{
		Commits: config.CommitsConfig{
			Prefixes: []string{"feat", "fix", "refactor", "docs", "test", "chore"},
		},
	}

	if err := ValidatePrefix(cfg, "feat"); err != nil {
		t.Errorf("valid prefix 'feat' failed: %v", err)
	}
	if err := ValidatePrefix(cfg, "invalid"); err == nil {
		t.Error("expected error for invalid prefix")
	}
}

func TestStripAttribution(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"no attribution",
			"feat(x): add something\n\nDetails here",
			"feat(x): add something\n\nDetails here",
		},
		{
			"co-authored-by",
			"feat(x): add something\n\nCo-Authored-By: Claude <claude@anthropic.com>",
			"feat(x): add something",
		},
		{
			"generated with",
			"feat(x): add something\n\nGenerated with Claude Code",
			"feat(x): add something",
		},
		{
			"model name in line",
			"feat(x): add something\n\nUsed GPT-4 for assistance",
			"feat(x): add something",
		},
		{
			"multiple attributions",
			"feat(x): add something\n\nCo-Authored-By: AI\nGenerated with Copilot\nSome real content",
			"feat(x): add something\n\nSome real content",
		},
		{
			"empty message",
			"",
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAttribution(tt.input)
			if got != tt.want {
				t.Errorf("StripAttribution() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestStripAttribution_KeepsLegitimateModelWords pins that subjects merely
// containing model/tool names as ordinary words are never stripped.
func TestStripAttribution_KeepsLegitimateModelWords(t *testing.T) {
	for _, msg := range []string{
		"fix(feat-0012): reset cursor position after paste",
		"feat(feat-0003): improve codex plan parsing",
		"refactor(fix-0001): rename claudeAdapter to surfaceAdapter",
	} {
		if got := StripAttribution(msg); got != msg {
			t.Errorf("StripAttribution(%q) = %q — legitimate subject was stripped", msg, got)
		}
	}
}
