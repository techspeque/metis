package config

import (
	"fmt"
	"time"
)

// SurfaceHuman is the surface of an agent that is a person: it has no
// launcher, and metis auto always stops before its sessions.
const SurfaceHuman = "human"

// ReviewConfig holds the review contract every handoff prompt and the
// generated instructions state.
type ReviewConfig struct {
	// NarrowAfter is the review cycle from which a reviewer may only
	// confirm or re-raise existing findings and open new blocking ones at
	// P1. Unset means 1; 0 disables narrowing.
	NarrowAfter *int `yaml:"narrow_after,omitempty" json:"narrow_after,omitempty"`
	// BlockReasons are the only grounds for a blocking finding; anything
	// else is recorded as an advisory. Unset means DefaultBlockReasons.
	BlockReasons []string `yaml:"block_reasons,omitempty" json:"block_reasons,omitempty"`
}

// DefaultBlockReasons apply when review.block_reasons is unset.
var DefaultBlockReasons = []string{
	"a deviation from the brief",
	"a missing or failing test the brief lists",
	"a broken accuracy rule",
	"a demonstrated defect",
}

// NarrowAfterCycles returns review.narrow_after with its default applied.
func (r *ReviewConfig) NarrowAfterCycles() int {
	if r.NarrowAfter == nil {
		return 1
	}
	return *r.NarrowAfter
}

// Narrowed reports whether a review that follows the given number of
// blocked reviews is narrowed.
func (r *ReviewConfig) Narrowed(reviewCycles int) bool {
	n := r.NarrowAfterCycles()
	return n > 0 && reviewCycles >= n
}

// Reasons returns review.block_reasons with its default applied.
func (r *ReviewConfig) Reasons() []string {
	if len(r.BlockReasons) == 0 {
		return DefaultBlockReasons
	}
	return r.BlockReasons
}

// Auto modes: how far one metis auto run takes a slice.
const (
	ModeManual = "manual" // the loop stops before the slice
	ModeStep   = "step"   // one slice to reviewed and archived, then stop
	ModeAuto   = "auto"   // continue until a stop rule fires
)

// Modes is the closed mode vocabulary.
var Modes = []string{ModeManual, ModeStep, ModeAuto}

// ValidMode reports whether m is one of Modes.
func ValidMode(m string) bool {
	for _, v := range Modes {
		if v == m {
			return true
		}
	}
	return false
}

// Branch modes: where unattended work lands.
const (
	BranchSlice = "slice" // auto/<slice-id>, fast-forward merged at reviewed
	BranchPR    = "pr"    // auto/<slice-id>, pushed with a pull request at reviewed
	BranchOff   = "off"   // commits on the work branch
)

// AutoConfig configures metis auto. Every key is optional.
type AutoConfig struct {
	Mode           string            `yaml:"mode,omitempty" json:"mode,omitempty"`
	Phases         map[string]string `yaml:"phases,omitempty" json:"phases,omitempty"`
	ByRisk         map[string]string `yaml:"by_risk,omitempty" json:"by_risk,omitempty"`
	Until          string            `yaml:"until,omitempty" json:"until,omitempty"`
	SessionTimeout string            `yaml:"session_timeout,omitempty" json:"session_timeout,omitempty"`
	Branch         string            `yaml:"branch,omitempty" json:"branch,omitempty"`
	PRCommand      []string          `yaml:"pr_command,omitempty" json:"pr_command,omitempty"`
	Notify         []string          `yaml:"notify,omitempty" json:"notify,omitempty"`
	Stop           AutoStopConfig    `yaml:"stop,omitempty" json:"stop,omitempty"`
}

// AutoStopConfig switches and bounds the stop rules. Unset switches are on.
type AutoStopConfig struct {
	Gate            *bool        `yaml:"gate,omitempty" json:"gate,omitempty"`
	MaxReviewCycles *int         `yaml:"max_review_cycles,omitempty" json:"max_review_cycles,omitempty"`
	RepeatFinding   *bool        `yaml:"repeat_finding,omitempty" json:"repeat_finding,omitempty"`
	GrowingFindings *bool        `yaml:"growing_findings,omitempty" json:"growing_findings,omitempty"`
	P1              *bool        `yaml:"p1,omitempty" json:"p1,omitempty"`
	NeedsHuman      *bool        `yaml:"needs_human,omitempty" json:"needs_human,omitempty"`
	VerifyEnv       *bool        `yaml:"verify_env,omitempty" json:"verify_env,omitempty"`
	Scope           *bool        `yaml:"scope,omitempty" json:"scope,omitempty"`
	DirtyTree       *bool        `yaml:"dirty_tree,omitempty" json:"dirty_tree,omitempty"`
	Planning        *bool        `yaml:"planning,omitempty" json:"planning,omitempty"`
	SessionFailed   *int         `yaml:"session_failed,omitempty" json:"session_failed,omitempty"`
	Budget          BudgetConfig `yaml:"budget,omitempty" json:"budget,omitempty"`
}

// BudgetConfig bounds the work one slice may consume between stops.
type BudgetConfig struct {
	WallClock string `yaml:"wall_clock,omitempty" json:"wall_clock,omitempty"`
	Sessions  *int   `yaml:"sessions,omitempty" json:"sessions,omitempty"`
}

// On reports whether a stop switch is on: unset means on.
func On(b *bool) bool {
	return b == nil || *b
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func durationOr(s string, def time.Duration) (time.Duration, error) {
	if s == "" {
		return def, nil
	}
	return time.ParseDuration(s)
}

// ProjectMode returns auto.mode, or "" when unset.
func (a *AutoConfig) ProjectMode() string {
	return a.Mode
}

// UntilTarget returns auto.until with its default (gate) applied.
func (a *AutoConfig) UntilTarget() string {
	if a.Until == "" {
		return "gate"
	}
	return a.Until
}

// BranchMode returns auto.branch with its default (slice) applied.
func (a *AutoConfig) BranchMode() string {
	if a.Branch == "" {
		return BranchSlice
	}
	return a.Branch
}

// PRArgv returns auto.pr_command with its default applied.
func (a *AutoConfig) PRArgv() []string {
	if len(a.PRCommand) == 0 {
		return []string{"gh", "pr", "create", "--fill"}
	}
	return a.PRCommand
}

// Timeout returns auto.session_timeout with its default (2h) applied.
func (a *AutoConfig) Timeout() time.Duration {
	d, err := durationOr(a.SessionTimeout, 2*time.Hour)
	if err != nil || d <= 0 {
		return 2 * time.Hour
	}
	return d
}

// MaxReviews returns auto.stop.max_review_cycles (default 3).
func (s *AutoStopConfig) MaxReviews() int {
	return intOr(s.MaxReviewCycles, 3)
}

// Retries returns how many times a failed or idle session is retried
// (auto.stop.session_failed, default 1).
func (s *AutoStopConfig) Retries() int {
	return intOr(s.SessionFailed, 1)
}

// BudgetWallClock returns the per-slice wall-clock budget (default 6h).
func (s *AutoStopConfig) BudgetWallClock() time.Duration {
	d, err := durationOr(s.Budget.WallClock, 6*time.Hour)
	if err != nil || d <= 0 {
		return 6 * time.Hour
	}
	return d
}

// BudgetSessions returns the per-slice session budget (default 8).
func (s *AutoStopConfig) BudgetSessions() int {
	return intOr(s.Budget.Sessions, 8)
}

// validateAuto checks the auto and review sections.
func (c *Config) validateAuto() []error {
	var errs []error
	a := &c.Auto
	if a.Mode != "" && !ValidMode(a.Mode) {
		errs = append(errs, fmt.Errorf("auto.mode %q is not a mode (valid: manual, step, auto)", a.Mode))
	}
	for phase, m := range a.Phases {
		if !ValidMode(m) {
			errs = append(errs, fmt.Errorf("auto.phases.%s %q is not a mode (valid: manual, step, auto)", phase, m))
		}
	}
	for risk, m := range a.ByRisk {
		switch risk {
		case "low", "medium", "high":
		default:
			errs = append(errs, fmt.Errorf("auto.by_risk: %q is not a risk (valid: low, medium, high)", risk))
		}
		if !ValidMode(m) {
			errs = append(errs, fmt.Errorf("auto.by_risk.%s %q is not a mode (valid: manual, step, auto)", risk, m))
		}
	}
	switch a.Branch {
	case "", BranchSlice, BranchPR, BranchOff:
	default:
		errs = append(errs, fmt.Errorf("auto.branch %q is not valid (valid: slice, pr, off)", a.Branch))
	}
	if err := ValidUntil(a.Until); a.Until != "" && err != nil {
		errs = append(errs, fmt.Errorf("auto.until: %w", err))
	}
	for key, v := range map[string]string{
		"auto.session_timeout":        a.SessionTimeout,
		"auto.stop.budget.wall_clock": a.Stop.Budget.WallClock,
	} {
		if v == "" {
			continue
		}
		if d, err := time.ParseDuration(v); err != nil || d <= 0 {
			errs = append(errs, fmt.Errorf("%s %q is not a positive duration (e.g. 2h, 90m)", key, v))
		}
	}
	for key, p := range map[string]*int{
		"auto.stop.max_review_cycles": a.Stop.MaxReviewCycles,
		"auto.stop.session_failed":    a.Stop.SessionFailed,
		"auto.stop.budget.sessions":   a.Stop.Budget.Sessions,
		"review.narrow_after":         c.Review.NarrowAfter,
	} {
		if p != nil && *p < 0 {
			errs = append(errs, fmt.Errorf("%s must not be negative", key))
		}
	}
	for slug, agent := range c.Agents {
		if agent.Surface == SurfaceHuman && len(agent.Launch) > 0 {
			errs = append(errs, fmt.Errorf("agent %q: a human agent has no launch command", slug))
		}
	}
	return errs
}

// ValidUntil checks an auto.until / --until value: gate, phase, empty, or
// slice:<id>.
func ValidUntil(u string) error {
	switch {
	case u == "gate", u == "phase", u == "empty":
		return nil
	case len(u) > len("slice:") && u[:len("slice:")] == "slice:":
		return nil
	}
	return fmt.Errorf("%q is not a target (valid: gate, phase, empty, slice:<id>)", u)
}
