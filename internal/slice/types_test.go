package slice

import (
	"testing"
)

func TestWorkType_IsValid(t *testing.T) {
	tests := []struct {
		wt   WorkType
		want bool
	}{
		{TypeFeat, true},
		{TypeFix, true},
		{TypeRefactor, true},
		{TypeDebt, true},
		{TypeRemove, true},
		{TypeChore, true},
		{TypeSecurity, true},
		{TypeGate, true},
		{TypeRecon, true},
		{"invalid", false},
		{"", false},
		{"FEAT", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.wt), func(t *testing.T) {
			if got := tt.wt.IsValid(); got != tt.want {
				t.Errorf("WorkType(%q).IsValid() = %v, want %v", tt.wt, got, tt.want)
			}
		})
	}
}

func TestWorkType_CommitPrefix(t *testing.T) {
	tests := []struct {
		wt   WorkType
		want string
	}{
		{TypeFeat, "feat"},
		{TypeFix, "fix"},
		{TypeRefactor, "refactor"},
		{TypeDebt, "refactor"},
		{TypeRemove, "refactor"},
		{TypeChore, "chore"},
		{TypeSecurity, "fix"},
		{TypeGate, "docs"},
		{TypeRecon, "docs"},
	}
	for _, tt := range tests {
		t.Run(string(tt.wt), func(t *testing.T) {
			if got := tt.wt.CommitPrefix(); got != tt.want {
				t.Errorf("WorkType(%q).CommitPrefix() = %q, want %q", tt.wt, got, tt.want)
			}
		})
	}
}

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		p    Priority
		want bool
	}{
		{PriorityP0, true},
		{PriorityP1, true},
		{PriorityP2, true},
		{PriorityP3, true},
		{"p4", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			if got := tt.p.IsValid(); got != tt.want {
				t.Errorf("Priority(%q).IsValid() = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestPriority_Rank(t *testing.T) {
	tests := []struct {
		p    Priority
		want int
	}{
		{PriorityP0, 0},
		{PriorityP1, 1},
		{PriorityP2, 2},
		{PriorityP3, 3},
	}
	for _, tt := range tests {
		t.Run(string(tt.p), func(t *testing.T) {
			if got := tt.p.Rank(); got != tt.want {
				t.Errorf("Priority(%q).Rank() = %d, want %d", tt.p, got, tt.want)
			}
		})
	}
	// Verify ordering
	if PriorityP0.Rank() >= PriorityP1.Rank() {
		t.Error("p0 should rank lower (more urgent) than p1")
	}
}

func TestRisk_IsValid(t *testing.T) {
	tests := []struct {
		r    Risk
		want bool
	}{
		{RiskLow, true},
		{RiskMedium, true},
		{RiskHigh, true},
		{"critical", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			if got := tt.r.IsValid(); got != tt.want {
				t.Errorf("Risk(%q).IsValid() = %v, want %v", tt.r, got, tt.want)
			}
		})
	}
}

func TestSlice_Status(t *testing.T) {
	tests := []struct {
		name  string
		slice Slice
		want  Status
	}{
		{"pending", Slice{Coded: false, Reviewed: false}, StatusPending},
		{"reviewing", Slice{Coded: true, Reviewed: false}, StatusReviewing},
		{"done", Slice{Coded: true, Reviewed: true}, StatusDone},
		{"rework", Slice{Coded: false, Reviewed: false, ReviewCycles: 1}, StatusRework},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.slice.Status(); got != tt.want {
				t.Errorf("Slice.Status() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlice_ActiveRole(t *testing.T) {
	tests := []struct {
		name  string
		slice Slice
		want  Role
	}{
		{"needs coding", Slice{Coded: false, Reviewed: false}, RoleCoder},
		{"needs review", Slice{Coded: true, Reviewed: false}, RoleReviewer},
		{"done", Slice{Coded: true, Reviewed: true}, ""},
		{"rework", Slice{Coded: false, Reviewed: false, ReviewCycles: 1}, RoleCoder},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.slice.ActiveRole(); got != tt.want {
				t.Errorf("Slice.ActiveRole() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSlice_Validate(t *testing.T) {
	valid := Slice{
		ID:       "feat-0001",
		Title:    "Test slice",
		Type:     TypeFeat,
		Priority: PriorityP2,
		Risk:     RiskMedium,
		Coder:    "opencode/opus",
		Reviewer: "codex",
		Created:  "2026-07-09",
	}

	if errs := valid.Validate(); len(errs) != 0 {
		t.Errorf("valid slice has errors: %v", errs)
	}

	// Test invalid: same coder and reviewer
	s := valid
	s.Reviewer = s.Coder
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("expected error for coder == reviewer")
	}

	// Test invalid: reviewed but not coded
	s = valid
	s.Coded = false
	s.Reviewed = true
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("expected error for reviewed && !coded")
	}

	// Test invalid: plan set without plan_section
	s = valid
	s.Plan = "plans/impl.md"
	s.PlanSection = ""
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("expected error for plan without plan_section")
	}

	// Test invalid: bad type
	s = valid
	s.Type = "invalid"
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("expected error for invalid type")
	}

	// Test invalid: empty ID
	s = valid
	s.ID = ""
	if errs := s.Validate(); len(errs) == 0 {
		t.Error("expected error for empty ID")
	}
}
