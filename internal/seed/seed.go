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
	Phase      int
	Workstream string
	Title      string
	Risk       slice.Risk
	Coder      string
	Reviewer   string
	Stage      string
	Tasks      []string
	Acceptance []string
}

// ParseResult holds the output of parsing a plan file.
type ParseResult struct {
	Workstreams []Workstream
	// GatePhases lists phases whose plan contains a "Phase Gate" section;
	// each produces an auto-generated gate slice blocked by that phase's
	// workstreams.
	GatePhases []int
	Errors     []string
}

// Heading forms accepted: the shipped template puts the phase in the H1
// ("# Phase 0 — Foundation") or frontmatter ("phase: 0") and workstreams at
// H2 ("## Workstream 0.1: Title"); older hand-written plans used "## Phase 0"
// with H3 workstreams. All of these parse.
var phaseHeadingRe = regexp.MustCompile(`^#{1,3}\s+Phase\s+(\d+)\b`)
var wsHeadingRe = regexp.MustCompile(`^#{2,3}\s+Workstream\s+(\d+\.\d+):\s*(.+)`)
var metadataRe = regexp.MustCompile(`^-\s+\*\*(\w+):\*\*\s*(.+)`)
var frontmatterPhaseRe = regexp.MustCompile(`^phase:\s*(\d+)\s*$`)
var gateHeadingRe = regexp.MustCompile(`^#{2,3}\s+Phase Gate\b`)

// Parse parses a structured plan file and extracts workstreams.
func Parse(content string) *ParseResult {
	result := &ParseResult{}
	lines := strings.Split(content, "\n")

	var currentPhase int
	var currentWS *Workstream
	inTasks := false
	inAcceptance := false
	inFrontmatter := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Frontmatter phase ("phase: N" between the leading --- fences)
		if trimmed == "---" && (i == 0 || inFrontmatter) {
			inFrontmatter = i == 0
			continue
		}
		if inFrontmatter {
			if m := frontmatterPhaseRe.FindStringSubmatch(trimmed); m != nil {
				_, _ = fmt.Sscanf(m[1], "%d", &currentPhase)
			}
			continue
		}

		// Phase heading
		if m := phaseHeadingRe.FindStringSubmatch(trimmed); m != nil {
			_, _ = fmt.Sscanf(m[1], "%d", &currentPhase)
			if currentWS != nil {
				result.Workstreams = append(result.Workstreams, *currentWS)
				currentWS = nil
			}
			inTasks = false
			inAcceptance = false
			continue
		}

		// Phase Gate heading — becomes an auto-generated gate slice
		if gateHeadingRe.MatchString(trimmed) {
			if currentWS != nil {
				result.Workstreams = append(result.Workstreams, *currentWS)
				currentWS = nil
			}
			seen := false
			for _, p := range result.GatePhases {
				if p == currentPhase {
					seen = true
				}
			}
			if !seen {
				result.GatePhases = append(result.GatePhases, currentPhase)
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

// ToSlices converts parsed workstreams (and Phase Gate sections) into slice
// entries. Each gate slice is blocked by every workstream of its phase and
// inherits the phase's first coder/reviewer pair.
func ToSlices(result *ParseResult, planFile string, sliceType slice.WorkType) []slice.Slice {
	ws := result.Workstreams
	var slices []slice.Slice
	byPhase := map[int][]string{}
	firstPair := map[int][2]string{}
	for i := range ws {
		w := &ws[i]
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
		byPhase[w.Phase] = append(byPhase[w.Phase], id)
		if _, ok := firstPair[w.Phase]; !ok {
			firstPair[w.Phase] = [2]string{w.Coder, w.Reviewer}
		}
	}

	for _, phase := range result.GatePhases {
		deps, ok := byPhase[phase]
		if !ok {
			continue
		}
		pair := firstPair[phase]
		slices = append(slices, slice.Slice{
			ID:          fmt.Sprintf("phase-%d-gate", phase),
			Title:       fmt.Sprintf("Phase %d gate: composed-system validation", phase),
			Type:        slice.TypeGate,
			Priority:    slice.PriorityP2,
			Risk:        slice.RiskHigh,
			Coder:       pair[0],
			Reviewer:    pair[1],
			Plan:        planFile,
			PlanSection: "Phase Gate",
			BlockedBy:   deps,
		})
	}
	return slices
}
