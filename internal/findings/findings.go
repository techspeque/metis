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
	Status     string `yaml:"status" json:"status"`                               // open, resolved, promoted
	PromotedTo *int   `yaml:"promoted_to,omitempty" json:"promoted_to,omitempty"` // accuracy_rule index if promoted
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
	for _, f := range s.Findings {
		var n int
		if _, err := fmt.Sscanf(f.ID, "f-%d", &n); err == nil && n > maxID {
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

// FindByID returns a pointer to a finding by ID.
func (s *Store) FindByID(id string) *Finding {
	for i := range s.Findings {
		if s.Findings[i].ID == id {
			return &s.Findings[i]
		}
	}
	return nil
}

// Filter returns findings matching the given criteria (empty string = no filter).
func (s *Store) Filter(severity, category, sliceID string) []Finding {
	var result []Finding
	for _, f := range s.Findings {
		if severity != "" && f.Severity != severity {
			continue
		}
		if category != "" && f.Category != category {
			continue
		}
		if sliceID != "" && f.Slice != sliceID {
			continue
		}
		result = append(result, f)
	}
	return result
}

// Stats returns aggregated statistics about findings.
type Stats struct {
	Total      int                   `json:"total"`
	BySeverity map[string]int        `json:"by_severity,omitempty"`
	ByCategory map[string]int        `json:"by_category,omitempty"`
	ByAgent    map[string]AgentStats `json:"by_agent,omitempty"`
}

// AgentStats holds per-agent finding statistics.
type AgentStats struct {
	Slices    int `json:"slices"`
	Blocks    int `json:"blocks"`
	FirstPass int `json:"first_pass"` // slices with 0 blocks
}

// GetStats computes aggregate statistics.
func (s *Store) GetStats() Stats {
	stats := Stats{
		Total:      len(s.Findings),
		BySeverity: make(map[string]int),
		ByCategory: make(map[string]int),
		ByAgent:    make(map[string]AgentStats),
	}
	for _, f := range s.Findings {
		stats.BySeverity[f.Severity]++
		stats.ByCategory[f.Category]++
	}
	return stats
}

// OpenFindings returns all findings with status "open".
func (s *Store) OpenFindings() []Finding {
	return s.Filter("", "", "")
}
