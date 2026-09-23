// Package findings manages the review findings store (.metis/findings.yaml)
// and provides aggregation, filtering, and rules promotion.
package findings

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/techspeque/metis/internal/fsutil"
)

// Finding represents a single review finding.
type Finding struct {
	ID         string `yaml:"id" json:"id"`
	Date       string `yaml:"date" json:"date"`
	Slice      string `yaml:"slice" json:"slice"`
	Severity   string `yaml:"severity" json:"severity"` // P1, P2, P3
	Category   string `yaml:"category" json:"category"` // auth, protocol, scope, tests, arch-dup, arch-fit, data, maint, security, behavior, performance
	Finding    string `yaml:"finding" json:"finding"`
	Status     string `yaml:"status" json:"status"`                               // open, advisory, resolved, promoted
	PromotedTo *int   `yaml:"promoted_to,omitempty" json:"promoted_to,omitempty"` // accuracy_rule index if promoted
	ResolvedBy string `yaml:"resolved_by,omitempty" json:"resolved_by,omitempty"` // note/commit recorded at resolution
}

// Store holds all findings.
type Store struct {
	Findings []Finding `yaml:"findings"`
}

// ValidSeverities are the allowed severity levels.
var ValidSeverities = []string{"P1", "P2", "P3"}

// ValidCategories are the allowed finding categories.
var ValidCategories = []string{
	"auth", "protocol", "scope", "tests", "arch-dup",
	"arch-fit", "data", "maint", "security", "behavior", "performance",
}

// Load reads the findings store from a YAML file.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{Findings: []Finding{}}, nil
		}
		return nil, fmt.Errorf("reading findings: %w", err)
	}
	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing findings: %w", err)
	}
	if s.Findings == nil {
		s.Findings = []Finding{}
	}
	return &s, nil
}

// Save writes the findings store to a YAML file.
func (s *Store) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating findings directory: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshaling findings: %w", err)
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// Add appends a new finding and returns its generated ID. The ID is one
// past the highest existing numeric ID (not the slice length), so IDs stay
// unique even after findings are compacted or appended concurrently.
func (s *Store) Add(sliceID, severity, category, finding string) string {
	maxID := 0
	for i := range s.Findings {
		var n int
		if _, err := fmt.Sscanf(s.Findings[i].ID, "f-%d", &n); err == nil && n > maxID {
			maxID = n
		}
	}
	id := fmt.Sprintf("f-%03d", maxID+1)
	s.Findings = append(s.Findings, Finding{
		ID:       id,
		Date:     time.Now().Format("2006-01-02"),
		Slice:    sliceID,
		Severity: severity,
		Category: category,
		Finding:  finding,
		Status:   "open",
	})
	return id
}

// AddWithStatus appends a finding with an explicit status (e.g. "advisory"
// for non-blocking observations) and returns its ID.
func (s *Store) AddWithStatus(sliceID, severity, category, finding, status string) string {
	id := s.Add(sliceID, severity, category, finding)
	s.FindByID(id).Status = status
	return id
}

// Resolve marks a finding resolved, recording how.
func (s *Store) Resolve(id, resolvedBy string) error {
	f := s.FindByID(id)
	if f == nil {
		return fmt.Errorf("finding %q not found", id)
	}
	if f.Status == "resolved" || f.Status == "promoted" {
		return fmt.Errorf("finding %q is already %s", id, f.Status)
	}
	f.Status = "resolved"
	f.ResolvedBy = resolvedBy
	return nil
}

// FindByID returns a pointer to a finding by ID.
func (s *Store) FindByID(id string) *Finding {
	for i := range s.Findings {
		if s.Findings[i].ID == id {
			return &s.Findings[i]
		}
	}
	return nil
}

// Statuses is the closed status vocabulary, in lifecycle order.
var Statuses = []string{"open", "advisory", "resolved", "promoted"}

// ValidStatus reports whether status is one of Statuses.
func ValidStatus(status string) bool {
	for _, s := range Statuses {
		if s == status {
			return true
		}
	}
	return false
}

// Filter returns findings matching the given criteria (empty string = no filter).
func (s *Store) Filter(severity, category, sliceID string) []Finding {
	return s.FilterStatus(severity, category, sliceID, "")
}

// FilterStatus is Filter with a status criterion as well.
func (s *Store) FilterStatus(severity, category, sliceID, status string) []Finding {
	var result []Finding
	for i := range s.Findings {
		f := &s.Findings[i]
		if severity != "" && f.Severity != severity {
			continue
		}
		if category != "" && f.Category != category {
			continue
		}
		if sliceID != "" && f.Slice != sliceID {
			continue
		}
		if status != "" && f.Status != status {
			continue
		}
		result = append(result, *f)
	}
	return result
}

// Stats returns aggregated statistics about findings.
type Stats struct {
	Total      int                   `json:"total"`
	BySeverity map[string]int        `json:"by_severity,omitempty"`
	ByCategory map[string]int        `json:"by_category,omitempty"`
	ByStatus   map[string]int        `json:"by_status,omitempty"`
	ByAgent    map[string]AgentStats `json:"by_agent,omitempty"`
}

// AgentStats holds per-agent finding statistics.
type AgentStats struct {
	Slices    int `json:"slices"`
	Done      int `json:"done"`
	Blocks    int `json:"blocks"`
	FirstPass int `json:"first_pass"` // done slices with 0 review cycles
}

// GetStats computes aggregate statistics.
func (s *Store) GetStats() Stats {
	stats := Stats{
		Total:      len(s.Findings),
		BySeverity: make(map[string]int),
		ByCategory: make(map[string]int),
		ByStatus:   make(map[string]int),
		ByAgent:    make(map[string]AgentStats),
	}
	for i := range s.Findings {
		stats.BySeverity[s.Findings[i].Severity]++
		stats.ByCategory[s.Findings[i].Category]++
		stats.ByStatus[s.Findings[i].Status]++
	}
	return stats
}

// OpenFindings returns all findings with status "open".
func (s *Store) OpenFindings() []Finding {
	return s.FilterStatus("", "", "", "open")
}

// RenumberRules keeps promoted findings pointing at the right accuracy rule
// after the rules numbered in removed (1-based) are deleted: a finding
// promoted to a removed rule loses its pointer but stays promoted, as a
// record that it once was; one promoted to a later rule shifts down. It
// reports whether any finding changed.
func (s *Store) RenumberRules(removed []int) bool {
	gone := make(map[int]bool, len(removed))
	for _, n := range removed {
		gone[n] = true
	}
	changed := false
	for i := range s.Findings {
		p := s.Findings[i].PromotedTo
		if p == nil {
			continue
		}
		if gone[*p] {
			s.Findings[i].PromotedTo = nil
			changed = true
			continue
		}
		shift := 0
		for n := range gone {
			if n < *p {
				shift++
			}
		}
		if shift > 0 {
			next := *p - shift
			s.Findings[i].PromotedTo = &next
			changed = true
		}
	}
	return changed
}
