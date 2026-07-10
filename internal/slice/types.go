// Package slice defines the core domain types for Metis work slices.
package slice

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

// Priority represents the urgency level of a slice.
type Priority string

const (
	PriorityP0 Priority = "p0" // Drop everything
	PriorityP1 Priority = "p1" // Next up
	PriorityP2 Priority = "p2" // Normal (default)
	PriorityP3 Priority = "p3" // Backlog
)

// ValidPriorities is the set of all valid priority levels.
var ValidPriorities = []Priority{PriorityP0, PriorityP1, PriorityP2, PriorityP3}

// Risk represents the risk level of a slice.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// ValidRisks is the set of all valid risk levels.
var ValidRisks = []Risk{RiskLow, RiskMedium, RiskHigh}

// Slice is the fundamental unit of work in Metis.
type Slice struct {
	ID           string   `yaml:"id"`
	Title        string   `yaml:"title"`
	Type         WorkType `yaml:"type"`
	Priority     Priority `yaml:"priority"`
	Risk         Risk     `yaml:"risk"`
	Stage        string   `yaml:"stage,omitempty"`
	Coder        string   `yaml:"coder"`
	Reviewer     string   `yaml:"reviewer"`
	Plan         string   `yaml:"plan,omitempty"`
	PlanSection  string   `yaml:"plan_section,omitempty"`
	Coded        bool     `yaml:"coded"`
	Reviewed     bool     `yaml:"reviewed"`
	ReviewCycles int      `yaml:"review_cycles"`
	BlockedBy    []string `yaml:"blocked_by,omitempty"`
	Notes        string   `yaml:"notes,omitempty"`
	Created      string   `yaml:"created"`
}

// Role represents the role an agent plays for a given slice.
type Role string

const (
	RoleCoder    Role = "Coder"
	RoleReviewer Role = "Reviewer"
)
