// Package findings manages the review findings store (.metis/findings.yaml)
// and provides aggregation, filtering, and rules promotion.
package findings

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	Status     string `yaml:"status" json:"status"`                               // open, advisory, resolved, waived, promoted
	PromotedTo *int   `yaml:"promoted_to,omitempty" json:"promoted_to,omitempty"` // accuracy_rule index if promoted
	ResolvedBy string `yaml:"resolved_by,omitempty" json:"resolved_by,omitempty"` // note/commit recorded at resolution
	// Cycle is the review (1-based) that raised the finding; 0 on findings
	// recorded before metis tracked it.
	Cycle int `yaml:"cycle,omitempty" json:"cycle,omitempty"`
	// File is the repository path the finding is about: given with --file,
	// or the first file:line reference in its text.
	File string `yaml:"file,omitempty" json:"file,omitempty"`
	// FollowOn names the earlier finding this one follows: a recurrence, or
	// a fix that did not finish the job. Empty on a finding raised fresh.
	FollowOn string `yaml:"follow_on,omitempty" json:"follow_on,omitempty"`
	// Commit and Evidence are a code fix's proof: the full hash of the
	// fixing commit and the test name or file:line that shows it.
	Commit   string `yaml:"commit,omitempty" json:"commit,omitempty"`
	Evidence string `yaml:"evidence,omitempty" json:"evidence,omitempty"`
	// Disposition closes a finding without a fix: "duplicate" or
	// "superseded", with Of naming the finding that carries it instead.
	Disposition string `yaml:"disposition,omitempty" json:"disposition,omitempty"`
	Of          string `yaml:"of,omitempty" json:"of,omitempty"`
	// Waiver records who accepted the finding as it stands, and on what
	// terms. Severity and history stay as raised.
	Waiver *Waiver `yaml:"waiver,omitempty" json:"waiver,omitempty"`
	// Original is the classification as raised, kept from the first
	// correction on; Corrections is every correction since, in order.
	Original    *Classification `yaml:"original,omitempty" json:"original,omitempty"`
	Corrections []Correction    `yaml:"corrections,omitempty" json:"corrections,omitempty"`
	// Excluded takes a row out of the severity and category counts (an
	// aggregator such as "cycle 3 blocked on f-12..f-15", or a row recorded
	// before severities meant anything) while it stays in the ledger.
	Excluded bool `yaml:"excluded,omitempty" json:"excluded,omitempty"`
}

// Waiver is a finding accepted as it stands.
type Waiver struct {
	By       string `yaml:"by" json:"by"`
	Date     string `yaml:"date" json:"date"`
	Reason   string `yaml:"reason" json:"reason"`
	Residual string `yaml:"residual,omitempty" json:"residual,omitempty"`
	Owner    string `yaml:"owner,omitempty" json:"owner,omitempty"`
	Revisit  string `yaml:"revisit,omitempty" json:"revisit,omitempty"`
}

// Classification is a finding's severity and category.
type Classification struct {
	Severity string `yaml:"severity" json:"severity"`
	Category string `yaml:"category" json:"category"`
}

// Correction is one change to a finding's classification, with its
// provenance. Severity and Category are the values set, empty when left.
type Correction struct {
	Date     string `yaml:"date" json:"date"`
	By       string `yaml:"by" json:"by"`
	Reason   string `yaml:"reason" json:"reason"`
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`
	Category string `yaml:"category,omitempty" json:"category,omitempty"`
	Excluded bool   `yaml:"excluded,omitempty" json:"excluded,omitempty"`
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

// Dispositions are the ways a finding closes without a fix of its own.
var Dispositions = []string{"duplicate", "superseded"}

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
		Date:     today(),
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

// Follow links a finding to the earlier one it follows. The parent must
// exist and must not be the finding itself.
func (s *Store) Follow(id, parent string) error {
	f := s.FindByID(id)
	if f == nil {
		return fmt.Errorf("finding %q not found", id)
	}
	if parent == id {
		return fmt.Errorf("finding %q cannot follow itself", id)
	}
	if s.FindByID(parent) == nil {
		return fmt.Errorf("follow-on target %q not found", parent)
	}
	f.FollowOn = parent
	return nil
}

// Chain returns the findings a finding follows, nearest first, stopping at
// a cycle.
func (s *Store) Chain(id string) []string {
	var out []string
	seen := map[string]bool{id: true}
	for f := s.FindByID(id); f != nil && f.FollowOn != "" && !seen[f.FollowOn]; f = s.FindByID(f.FollowOn) {
		seen[f.FollowOn] = true
		out = append(out, f.FollowOn)
	}
	return out
}

// Resolve marks a finding fixed. commit is the fixing commit's hash as the
// caller resolved it; evidence is the test name or file:line that shows
// the fix. Neither may be empty: a resolution without evidence is a
// claim, and the ledger records fixes.
func (s *Store) Resolve(id, commit, evidence, note string) error {
	f, err := s.closable(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(commit) == "" || strings.TrimSpace(evidence) == "" {
		return fmt.Errorf("resolving %s needs the fixing commit and its evidence (a test name or file:line)", id)
	}
	f.Status = "resolved"
	f.Commit = commit
	f.Evidence = evidence
	f.ResolvedBy = fmt.Sprintf("commit %s; %s", shortHash(commit), evidence)
	if note != "" {
		f.ResolvedBy += "; " + note
	}
	return nil
}

// Dispose closes a finding as a duplicate of, or superseded by, another
// finding that carries it instead.
func (s *Store) Dispose(id, disposition, of string) error {
	f, err := s.closable(id)
	if err != nil {
		return err
	}
	if !contains(Dispositions, disposition) {
		return fmt.Errorf("disposition must be one of %s", strings.Join(Dispositions, "|"))
	}
	if of == id {
		return fmt.Errorf("finding %q cannot be %s of itself", id, disposition)
	}
	if s.FindByID(of) == nil {
		return fmt.Errorf("finding %q not found", of)
	}
	f.Status = "resolved"
	f.Disposition = disposition
	f.Of = of
	f.ResolvedBy = disposition + " of " + of
	return nil
}

// Waive accepts a finding as it stands. Severity, category and the text
// stay as raised; the waiver records who, when and on what terms.
func (s *Store) Waive(id string, w *Waiver) error {
	f, err := s.closable(id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(w.By) == "" || strings.TrimSpace(w.Reason) == "" {
		return fmt.Errorf("waiving %s needs who accepts it (--by) and why (--reason)", id)
	}
	kept := *w
	if kept.Date == "" {
		kept.Date = today()
	}
	f.Status = "waived"
	f.Waiver = &kept
	return nil
}

// Reclassify corrects a finding's severity or category, or excludes it
// from the counts, keeping the classification as raised and the
// correction's provenance. A correction that changes nothing is refused.
func (s *Store) Reclassify(id string, c *Correction) error {
	f := s.FindByID(id)
	if f == nil {
		return fmt.Errorf("finding %q not found", id)
	}
	if strings.TrimSpace(c.By) == "" || strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("reclassifying %s needs who corrects it (--by) and why (--reason)", id)
	}
	if c.Severity != "" && !contains(ValidSeverities, c.Severity) {
		return fmt.Errorf("severity must be one of %s", strings.Join(ValidSeverities, "|"))
	}
	if c.Category != "" && !contains(ValidCategories, c.Category) {
		return fmt.Errorf("category must be one of %s", strings.Join(ValidCategories, "|"))
	}
	changes := (c.Severity != "" && c.Severity != f.Severity) ||
		(c.Category != "" && c.Category != f.Category) ||
		(c.Excluded && !f.Excluded)
	if !changes {
		return fmt.Errorf("reclassifying %s changes nothing (severity %s, category %q, excluded %v)", id, f.Severity, f.Category, f.Excluded)
	}
	if f.Original == nil {
		f.Original = &Classification{Severity: f.Severity, Category: f.Category}
	}
	kept := *c
	if kept.Date == "" {
		kept.Date = today()
	}
	if c.Severity != "" {
		f.Severity = c.Severity
	}
	if c.Category != "" {
		f.Category = c.Category
	}
	if c.Excluded {
		f.Excluded = true
	}
	f.Corrections = append(f.Corrections, kept)
	return nil
}

// closable returns the finding when it can still be closed.
func (s *Store) closable(id string) (*Finding, error) {
	f := s.FindByID(id)
	if f == nil {
		return nil, fmt.Errorf("finding %q not found", id)
	}
	if f.Closed() {
		return nil, fmt.Errorf("finding %q is already %s", id, f.Status)
	}
	return f, nil
}

// Closed reports whether the finding needs no further action: fixed,
// waived, or promoted to a rule.
func (f *Finding) Closed() bool {
	return f.Status == "resolved" || f.Status == "waived" || f.Status == "promoted"
}

// Blocking reports whether the finding was raised by a block (open, or
// later resolved or promoted) rather than recorded as an advisory.
func (f *Finding) Blocking() bool {
	return f.Status != "advisory"
}

// fileRefRe matches a path-like token whose extension starts with a
// letter, optionally followed by :line — the "file:line" a finding is
// asked to lead with.
var fileRefRe = regexp.MustCompile(`(?:[\w\-]+/)*[\w\-.]*\w\.[A-Za-z][A-Za-z0-9]*(?::\d+)?`)

// lineRefRe matches a file:line reference with its line number — the kind
// of detail that is true of one commit and stale by the next.
var lineRefRe = regexp.MustCompile(`\w\.[A-Za-z][A-Za-z0-9]*:\d+`)

// FileRef extracts the first file reference from a finding's text,
// without its line number. Empty when the text names no file.
func FileRef(text string) string {
	for _, m := range fileRefRe.FindAllString(text, -1) {
		if i := strings.LastIndex(m, ":"); i > 0 {
			m = m[:i]
		}
		if m == "e.g" || m == "i.e" {
			continue
		}
		return m
	}
	return ""
}

// HasLineRef reports whether text carries a file:line reference.
func HasLineRef(text string) bool {
	return lineRefRe.MatchString(text)
}

// ForSlice returns every finding of a slice, in recorded order.
func (s *Store) ForSlice(sliceID string) []Finding {
	return s.FilterStatus("", "", sliceID, "")
}

// Place records the review a finding belongs to and the file it names.
// file overrides the reference extracted from the text.
func (f *Finding) Place(cycle int, file string) {
	f.Cycle = cycle
	if file == "" {
		file = FileRef(f.Finding)
	}
	f.File = file
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
var Statuses = []string{"open", "advisory", "resolved", "waived", "promoted"}

// ValidStatus reports whether status is one of Statuses.
func ValidStatus(status string) bool {
	return contains(Statuses, status)
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

// Stats returns aggregated statistics about findings. Total is every row
// in the ledger; Counted leaves out the excluded rows, and the severity
// and category counts are over Counted. Waived is reported on its own so
// a pass is never read as a fix.
type Stats struct {
	Total       int                   `json:"total"`
	Counted     int                   `json:"counted"`
	Excluded    int                   `json:"excluded"`
	Waived      int                   `json:"waived"`
	Recurrences int                   `json:"recurrences"`
	BySeverity  map[string]int        `json:"by_severity,omitempty"`
	ByCategory  map[string]int        `json:"by_category,omitempty"`
	ByStatus    map[string]int        `json:"by_status,omitempty"`
	ByAgent     map[string]AgentStats `json:"by_agent,omitempty"`
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
		f := &s.Findings[i]
		stats.ByStatus[f.Status]++
		if f.Status == "waived" {
			stats.Waived++
		}
		if f.FollowOn != "" {
			stats.Recurrences++
		}
		if f.Excluded {
			stats.Excluded++
			continue
		}
		stats.Counted++
		stats.BySeverity[f.Severity]++
		stats.ByCategory[f.Category]++
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

func today() string {
	return time.Now().Format("2006-01-02")
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
