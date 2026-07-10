package slice

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// idPattern matches auto-generated slice IDs: type-NNNN (e.g., feat-0012, fix-0003).
var idPattern = regexp.MustCompile(`^([a-z]+)-(\d{4})$`)

// phasePattern matches phase-based IDs: phase-N-ws-N.N (e.g., phase-2-ws-2.3).
var phasePattern = regexp.MustCompile(`^phase-(\d+)-ws-(\d+\.\d+)$`)

// slugPattern matches any valid slice ID: lowercase alphanumeric + hyphens + dots.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// GenerateID creates a new auto-generated ID for the given work type and sequence number.
// Format: {type}-{NNNN} (e.g., feat-0012).
func GenerateID(wt WorkType, seq int) string {
	return fmt.Sprintf("%s-%04d", wt, seq)
}

// IsValidID returns true if the given string is a valid slice ID.
// Valid IDs are: auto-generated (type-NNNN), phase-based (phase-N-ws-N.N),
// or any slug matching [a-z0-9][a-z0-9._-]*.
func IsValidID(id string) bool {
	if id == "" {
		return false
	}
	return slugPattern.MatchString(id)
}

// ParseAutoID extracts the work type and sequence number from an auto-generated ID.
// Returns the type string, sequence number, and whether parsing succeeded.
func ParseAutoID(id string) (string, int, bool) {
	m := idPattern.FindStringSubmatch(id)
	if m == nil {
		return "", 0, false
	}
	seq, err := strconv.Atoi(m[2])
	if err != nil {
		return "", 0, false
	}
	return m[1], seq, true
}

// ParsePhaseID extracts the phase number and workstream from a phase-based ID.
// Returns the phase number, workstream string, and whether parsing succeeded.
func ParsePhaseID(id string) (int, string, bool) {
	m := phasePattern.FindStringSubmatch(id)
	if m == nil {
		return 0, "", false
	}
	phase, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, "", false
	}
	return phase, m[2], true
}

// NextSequence scans a list of slice IDs and returns the next auto-generated
// sequence number for the given work type. If no matching IDs exist, returns 1.
func NextSequence(ids []string, wt WorkType) int {
	prefix := string(wt) + "-"
	max := 0
	for _, id := range ids {
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		_, seq, ok := ParseAutoID(id)
		if ok && seq > max {
			max = seq
		}
	}
	return max + 1
}
