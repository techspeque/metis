// Package slice defines the core domain types for Metis work slices.
package slice

import "fmt"

// WorkType represents the kind of work a slice performs.
type WorkType string

const (
	TypeFeat     WorkType = "feat"
	TypeFix      WorkType = "fix"
	TypeRefactor WorkType = "refactor"
	TypeDebt     WorkType = "debt"
	TypeRemove   WorkType = "remove"
	TypeChore    WorkType = "chore"
	TypeSecurity WorkType = "security"
	TypeGate     WorkType = "gate"
	TypeRecon    WorkType = "recon"
)

// ValidWorkTypes is the set of all valid work types.
var ValidWorkTypes = []WorkType{
	TypeFeat, TypeFix, TypeRefactor, TypeDebt, TypeRemove,
	TypeChore, TypeSecurity, TypeGate, TypeRecon,
}

// String returns the string representation of a WorkType.
func (w WorkType) String() string {
	return string(w)
}

// IsValid returns true if the WorkType is one of the recognized values.
func (w WorkType) IsValid() bool {
	for _, v := range ValidWorkTypes {
		if w == v {
			return true
		}
	}
	return false
}

// CommitPrefix returns the default commit prefix for this work type.
// Used when the agent doesn't explicitly specify a prefix.
func (w WorkType) CommitPrefix() string {
	switch w {
	case TypeFeat:
		return "feat"
	case TypeFix:
		return "fix"
	case TypeRefactor:
		return "refactor"
	case TypeDebt:
		return "refactor"
	case TypeRemove:
		return "refactor"
	case TypeChore:
		return "chore"
	case TypeSecurity:
		return "fix"
	case TypeGate:
		return "docs"
	case TypeRecon:
		return "docs"
	default:
		return "chore"
	}
}

// Priority represents the urgency level of a slice.
type Priority string

const (
	PriorityP0 Priority = "p0" // Drop everything
	PriorityP1 Priority = "p1" // Next up
	PriorityP2 Priority = "p2" // Normal (default)
	PriorityP3 Priority = "p3" // Backlog
)

// ValidPriorities is the set of all valid priority levels, ordered by urgency.
var ValidPriorities = []Priority{PriorityP0, PriorityP1, PriorityP2, PriorityP3}

// String returns the string representation of a Priority.
func (p Priority) String() string {
	return string(p)
}

// IsValid returns true if the Priority is one of the recognized values.
func (p Priority) IsValid() bool {
	for _, v := range ValidPriorities {
		if p == v {
			return true
		}
	}
	return false
}

// Rank returns a numeric rank for priority comparison (lower = more urgent).
// p0 = 0, p1 = 1, p2 = 2, p3 = 3.
func (p Priority) Rank() int {
	switch p {
	case PriorityP0:
		return 0
	case PriorityP1:
		return 1
	case PriorityP2:
		return 2
	case PriorityP3:
		return 3
	default:
		return 99
	}
}

// Risk represents the risk level of a slice.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// ValidRisks is the set of all valid risk levels.
var ValidRisks = []Risk{RiskLow, RiskMedium, RiskHigh}

// String returns the string representation of a Risk.
func (r Risk) String() string {
	return string(r)
}

// IsValid returns true if the Risk is one of the recognized values.
func (r Risk) IsValid() bool {
	for _, v := range ValidRisks {
		if r == v {
			return true
		}
	}
	return false
}

// Slice is the fundamental unit of work in Metis.
type Slice struct {
	ID           string   `yaml:"id" json:"id"`
	Title        string   `yaml:"title" json:"title"`
	Type         WorkType `yaml:"type" json:"type"`
	Priority     Priority `yaml:"priority" json:"priority"`
	Risk         Risk     `yaml:"risk" json:"risk"`
	Stage        string   `yaml:"stage,omitempty" json:"stage,omitempty"`
	Coder        string   `yaml:"coder" json:"coder"`
	Reviewer     string   `yaml:"reviewer" json:"reviewer"`
	Plan         string   `yaml:"plan,omitempty" json:"plan,omitempty"`
	PlanSection  string   `yaml:"plan_section,omitempty" json:"plan_section,omitempty"`
	Coded        bool     `yaml:"coded" json:"coded"`
	Reviewed     bool     `yaml:"reviewed" json:"reviewed"`
	ReviewCycles int      `yaml:"review_cycles" json:"review_cycles"`
	BlockedBy    []string `yaml:"blocked_by,omitempty" json:"blocked_by,omitempty"`
	Notes        string   `yaml:"notes,omitempty" json:"notes,omitempty"`
	Removed      bool     `yaml:"removed,omitempty" json:"removed,omitempty"`
	Created      string   `yaml:"created" json:"created"`
}

// Status returns the computed lifecycle status of the slice.
func (s *Slice) Status() Status {
	if s.Removed {
		return StatusRemoved
	}
	if s.Coded && s.Reviewed {
		return StatusDone
	}
	if s.Coded && !s.Reviewed {
		return StatusReviewing
	}
	if !s.Coded && s.ReviewCycles > 0 {
		return StatusRework
	}
	return StatusPending
}

// ActiveRole returns which role should act on this slice next.
// Returns RoleCoder if coding is needed, RoleReviewer if review is needed.
// Returns empty string if the slice is done.
func (s *Slice) ActiveRole() Role {
	if s.Coded && s.Reviewed {
		return ""
	}
	if !s.Coded {
		return RoleCoder
	}
	return RoleReviewer
}

// IsDone returns true if both coded and reviewed are true.
func (s *Slice) IsDone() bool {
	return s.Coded && s.Reviewed
}

// Validate checks the slice for structural errors and returns all found.
func (s *Slice) Validate(allowSelfReview bool) []error {
	var errs []error

	if s.ID == "" {
		errs = append(errs, fmt.Errorf("slice ID is empty"))
	}
	if s.Title == "" {
		errs = append(errs, fmt.Errorf("%s: title is empty", s.ID))
	}
	if !s.Type.IsValid() {
		errs = append(errs, fmt.Errorf("%s: invalid type %q", s.ID, s.Type))
	}
	if !s.Priority.IsValid() {
		errs = append(errs, fmt.Errorf("%s: invalid priority %q", s.ID, s.Priority))
	}
	if !s.Risk.IsValid() {
		errs = append(errs, fmt.Errorf("%s: invalid risk %q", s.ID, s.Risk))
	}
	if s.Coder == "" {
		errs = append(errs, fmt.Errorf("%s: coder is empty", s.ID))
	}
	if s.Reviewer == "" {
		errs = append(errs, fmt.Errorf("%s: reviewer is empty", s.ID))
	}
	if !allowSelfReview && s.Coder != "" && s.Reviewer != "" && s.Coder == s.Reviewer {
		errs = append(errs, fmt.Errorf("%s: coder and reviewer are the same (%s) — set routing.review to \"self\" to allow single-agent projects", s.ID, s.Coder))
	}
	if s.Plan != "" && s.PlanSection == "" {
		errs = append(errs, fmt.Errorf("%s: plan is set but plan_section is missing", s.ID))
	}
	if s.Reviewed && !s.Coded {
		errs = append(errs, fmt.Errorf("%s: reviewed=true but coded=false (invalid state)", s.ID))
	}

	return errs
}

// Role represents the role an agent plays for a given slice.
type Role string

const (
	RoleCoder    Role = "Coder"
	RoleReviewer Role = "Reviewer"
)

// String returns the string representation of a Role.
func (r Role) String() string {
	return string(r)
}

// Status represents the computed lifecycle status of a slice.
type Status string

const (
	StatusPending   Status = "pending"
	StatusCoding    Status = "coding"
	StatusReviewing Status = "reviewing"
	StatusRework    Status = "rework"
	StatusDone      Status = "done"
	StatusRemoved   Status = "removed"
)

// String returns the string representation of a Status.
func (s Status) String() string {
	return string(s)
}
