// Package seed parses structured implementation plan files and generates
// ledger entries from workstream headings and metadata.
package seed

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/techspeque/metis/internal/slice"
)

// Workstream represents a parsed workstream from a plan file.
type Workstream struct {
	Phase       int
	Workstream  string
	Title       string
	Risk        slice.Risk
	Coder       string
	Reviewer    string
	Stage       string
	Tasks       []string
	Acceptance  []string
}

// ParseResult holds the output of parsing a plan file.
type ParseResult struct {
	Workstreams []Workstream
	Errors      []string
}

var phaseHeadingRe = regexp.MustCompile(`^##\s+Phase\s+(\d+)`)
var wsHeadingRe = regexp.MustCompile(`^###\s+Workstream\s+(\d+\.\d+):\s*(.+)`)
var metadataRe = regexp.MustCompile(`^-\s+\*\*(\w+):\*\*\s*(.+)`)

// Parse parses a structured plan file and extracts workstreams.
func Parse(content string) *ParseResult {
	result := &ParseResult{}
	lines := strings.Split(content, "\n")

	var currentPhase int
	var currentWS *Workstream
	inTasks := false
	inAcceptance := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Phase heading
		if m := phaseHeadingRe.FindStringSubmatch(trimmed); m != nil {
			fmt.Sscanf(m[1], "%d", &currentPhase)
			if currentWS != nil {
				result.Workstreams = append(result.Workstreams, *currentWS)
				currentWS = nil
			}
			inTasks = false
			inAcceptance = false
			continue
		}

		// Workstream heading
		if m := wsHeadingRe.FindStringSubmatch(trimmed); m != nil {
			if currentWS != nil {
				result.Workstreams = append(result.Workstreams, *currentWS)
			}
			currentWS = &Workstream{
				Phase:      currentPhase,
				Workstream: m[1],
				Title:      strings.TrimSpace(m[2]),
				Risk:       slice.RiskMedium, // default
			}
			inTasks = false
			inAcceptance = false
			continue
		}

		if currentWS == nil {
			continue
		}

		// Metadata lines
		if m := metadataRe.FindStringSubmatch(trimmed); m != nil {
			key := strings.ToLower(m[1])
			val := strings.TrimSpace(m[2])
			switch key {
			case "risk":
				currentWS.Risk = slice.Risk(strings.ToLower(val))
			case "coder":
				currentWS.Coder = val
			case "reviewer":
				currentWS.Reviewer = val
			case "stage":
				currentWS.Stage = val
			}
			continue
		}

		// Section markers
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "tasks:") {
			inTasks = true
			inAcceptance = false
			continue
		}
		if strings.HasPrefix(lower, "acceptance criteria:") || strings.HasPrefix(lower, "acceptance:") {
			inTasks = false
			inAcceptance = true
			continue
		}

		// List items
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimPrefix(trimmed, "- ")
			if inTasks {
				currentWS.Tasks = append(currentWS.Tasks, item)
			} else if inAcceptance {
				currentWS.Acceptance = append(currentWS.Acceptance, item)
			}
		}
	}

	// Don't forget the last workstream
	if currentWS != nil {
		result.Workstreams = append(result.Workstreams, *currentWS)
	}

	return result
}

// ToSlices converts parsed workstreams into slice entries.
func ToSlices(ws []Workstream, planFile string, sliceType slice.WorkType) []slice.Slice {
	var slices []slice.Slice
	for _, w := range ws {
		id := fmt.Sprintf("phase-%d-ws-%s", w.Phase, w.Workstream)
		s := slice.Slice{
			ID:          id,
			Title:       w.Title,
			Type:        sliceType,
			Priority:    slice.PriorityP2,
			Risk:        w.Risk,
			Stage:       w.Stage,
			Coder:       w.Coder,
			Reviewer:    w.Reviewer,
			Plan:        planFile,
			PlanSection: fmt.Sprintf("§%s", w.Workstream),
		}
		slices = append(slices, s)
	}
	return slices
}
