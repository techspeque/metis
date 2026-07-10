package slice

import "testing"

func TestGenerateID(t *testing.T) {
	tests := []struct {
		wt   WorkType
		seq  int
		want string
	}{
		{TypeFeat, 1, "feat-0001"},
		{TypeFix, 3, "fix-0003"},
		{TypeRefactor, 12, "refactor-0012"},
		{TypeChore, 100, "chore-0100"},
		{TypeSecurity, 9999, "security-9999"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := GenerateID(tt.wt, tt.seq); got != tt.want {
				t.Errorf("GenerateID(%q, %d) = %q, want %q", tt.wt, tt.seq, got, tt.want)
			}
		})
	}
}

func TestIsValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"feat-0001", true},
		{"phase-2-ws-2.3", true},
		{"fix-0003", true},
		{"my-custom-slice", true},
		{"debt-0001", true},
		{"phase-0-ws-0.1", true},
		{"a", true},
		{"", false},
		{"Has-Uppercase", false},
		{"-starts-with-dash", false},
		{"has spaces", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := IsValidID(tt.id); got != tt.want {
				t.Errorf("IsValidID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestParseAutoID(t *testing.T) {
	tests := []struct {
		id      string
		wantTyp string
		wantSeq int
		wantOk  bool
	}{
		{"feat-0001", "feat", 1, true},
		{"fix-0012", "fix", 12, true},
		{"refactor-0100", "refactor", 100, true},
		{"phase-2-ws-2.3", "", 0, false}, // not an auto ID
		{"invalid", "", 0, false},
		{"feat-abc", "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			typ, seq, ok := ParseAutoID(tt.id)
			if ok != tt.wantOk {
				t.Errorf("ParseAutoID(%q) ok = %v, want %v", tt.id, ok, tt.wantOk)
				return
			}
			if ok {
				if typ != tt.wantTyp {
					t.Errorf("ParseAutoID(%q) type = %q, want %q", tt.id, typ, tt.wantTyp)
				}
				if seq != tt.wantSeq {
					t.Errorf("ParseAutoID(%q) seq = %d, want %d", tt.id, seq, tt.wantSeq)
				}
			}
		})
	}
}

func TestParsePhaseID(t *testing.T) {
	tests := []struct {
		id        string
		wantPhase int
		wantWS    string
		wantOk    bool
	}{
		{"phase-2-ws-2.3", 2, "2.3", true},
		{"phase-0-ws-0.1", 0, "0.1", true},
		{"phase-10-ws-10.5", 10, "10.5", true},
		{"feat-0001", 0, "", false},
		{"invalid", 0, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			phase, ws, ok := ParsePhaseID(tt.id)
			if ok != tt.wantOk {
				t.Errorf("ParsePhaseID(%q) ok = %v, want %v", tt.id, ok, tt.wantOk)
				return
			}
			if ok {
				if phase != tt.wantPhase {
					t.Errorf("ParsePhaseID(%q) phase = %d, want %d", tt.id, phase, tt.wantPhase)
				}
				if ws != tt.wantWS {
					t.Errorf("ParsePhaseID(%q) ws = %q, want %q", tt.id, ws, tt.wantWS)
				}
			}
		})
	}
}

func TestNextSequence(t *testing.T) {
	ids := []string{"feat-0001", "feat-0002", "fix-0001", "feat-0005"}

	if got := NextSequence(ids, TypeFeat); got != 6 {
		t.Errorf("NextSequence(feat) = %d, want 6", got)
	}
	if got := NextSequence(ids, TypeFix); got != 2 {
		t.Errorf("NextSequence(fix) = %d, want 2", got)
	}
	if got := NextSequence(ids, TypeChore); got != 1 {
		t.Errorf("NextSequence(chore) = %d, want 1 (no existing)", got)
	}
}
