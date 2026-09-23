// Package progress renders the terminal progress dashboard showing
// per-phase progress bars, completion percentages, and quality stats.
package progress

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/techspeque/metis/internal/slice"
)

// Dashboard holds data for the progress display.
type Dashboard struct {
	Total     int                      `json:"total"`
	Done      int                      `json:"done"`
	Coding    int                      `json:"coding"`
	Reviewing int                      `json:"reviewing"`
	Pending   int                      `json:"pending"`
	Rework    int                      `json:"rework"`
	Removed   int                      `json:"removed,omitempty"`
	Active    *slice.Slice             `json:"active,omitempty"`
	ByStage   map[string]StageProgress `json:"by_stage,omitempty"`
	// StageOrder is the stages in the order the plan reached them: first
	// appearance over the archive then the ledger, which is the order the
	// phases were seeded and worked.
	StageOrder []string `json:"stage_order,omitempty"`
	// ByPhase groups slices by the plan that seeded them (the plan file's
	// name, or the id's phase prefix); PhaseOrder is numeric by the phase
	// number, unnumbered phases last in first appearance. A stage name
	// recurs across phases, so the per-phase view is the one that reads
	// in plan order.
	ByPhase    map[string]PhaseProgress `json:"by_phase,omitempty"`
	PhaseOrder []string                 `json:"phase_order,omitempty"`
}

// StageProgress holds progress for a single stage.
type StageProgress struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

// PhaseProgress holds progress for one plan's slices, and its stages in
// the order the plan reached them.
type PhaseProgress struct {
	Total      int                      `json:"total"`
	Done       int                      `json:"done"`
	Stages     map[string]StageProgress `json:"stages,omitempty"`
	StageOrder []string                 `json:"stage_order,omitempty"`
}

// unplanned is the phase of a slice no plan seeded (a recon, an ad-hoc add).
const unplanned = "(unplanned)"

var phasePrefix = regexp.MustCompile(`^(phase-\d+)`)

// phaseOf names the plan a slice belongs to.
func phaseOf(s *slice.Slice) string {
	if s.Plan != "" {
		base := filepath.Base(s.Plan)
		return strings.TrimSuffix(base, filepath.Ext(base))
	}
	if m := phasePrefix.FindString(s.ID); m != "" {
		return m
	}
	return unplanned
}

var firstNumber = regexp.MustCompile(`\d+`)

// sortPhases orders phases by their first number, unnumbered ones after
// every numbered one and among themselves by first appearance.
func sortPhases(order []string) {
	rank := func(name string) (int, bool) {
		m := firstNumber.FindString(name)
		if m == "" {
			return 0, false
		}
		n, err := strconv.Atoi(m)
		return n, err == nil
	}
	appearance := map[string]int{}
	for i, name := range order {
		appearance[name] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		na, oka := rank(a)
		nb, okb := rank(b)
		switch {
		case oka && okb && na != nb:
			return na < nb
		case oka != okb:
			return oka
		default:
			return appearance[a] < appearance[b]
		}
	})
}

// Compute builds a Dashboard from a slice list.
func Compute(slices []slice.Slice) *Dashboard {
	d := &Dashboard{
		ByStage: make(map[string]StageProgress),
		ByPhase: make(map[string]PhaseProgress),
	}

	for i := range slices {
		// Retired slices left the plan — they count toward nothing.
		if slices[i].Status() == slice.StatusRemoved {
			d.Removed++
			continue
		}
		d.Total++
		stage := slices[i].Stage
		if stage == "" {
			stage = "(none)"
		}
		sp, seen := d.ByStage[stage]
		if !seen {
			d.StageOrder = append(d.StageOrder, stage)
		}
		sp.Total++
		phase := phaseOf(&slices[i])
		pp, seenPhase := d.ByPhase[phase]
		if !seenPhase {
			d.PhaseOrder = append(d.PhaseOrder, phase)
			pp.Stages = make(map[string]StageProgress)
		}
		pp.Total++
		ps, seenStage := pp.Stages[stage]
		if !seenStage {
			pp.StageOrder = append(pp.StageOrder, stage)
		}
		ps.Total++

		switch slices[i].Status() {
		case slice.StatusDone:
			d.Done++
			sp.Done++
			pp.Done++
			ps.Done++
		case slice.StatusReviewing:
			d.Reviewing++
		case slice.StatusRework:
			d.Rework++
		default:
			d.Pending++
		}

		d.ByStage[stage] = sp
		pp.Stages[stage] = ps
		d.ByPhase[phase] = pp
	}
	sortPhases(d.PhaseOrder)

	return d
}

// View selects which breakdown a rendering shows under the overall line.
type View string

// The dashboard views: the summary counts, the per-phase breakdown, and
// the per-stage breakdown (the default).
const (
	ViewStats View = "stats"
	ViewPhase View = "phase"
	ViewStage View = "stage"
)

// DefaultWidth is the line width a view is laid out for when the caller
// has no terminal to measure.
const DefaultWidth = 100

// RenderView produces the overall line and the one breakdown the view
// names, laid out to fit width columns (DefaultWidth when width is not
// positive). A view with nothing to break down says so instead of
// printing an empty section.
func (d *Dashboard) RenderView(v View, width int) string {
	if width <= 0 {
		width = DefaultWidth
	}
	var b strings.Builder
	d.writeHeader(&b)
	switch v {
	case ViewStats:
		d.writeCounts(&b)
	case ViewPhase:
		if len(d.PhaseOrder) == 0 {
			b.WriteString("No phases to show.\n")
		} else {
			d.writePhases(&b, width)
		}
	default:
		if !d.hasStages() {
			b.WriteString("No slices have a stage.\n")
		} else {
			d.writeStages(&b)
		}
	}
	return b.String()
}

func (d *Dashboard) writeHeader(b *strings.Builder) {
	b.WriteString("═══ Metis Progress ═══\n\n")
	fmt.Fprintf(b, "Overall: %d/%d done (%.0f%%)\n", d.Done, d.Total, percent(d.Done, d.Total))
	b.WriteString(progressBar(d.Done, d.Total, 40))
	b.WriteString("\n\n")
}

func (d *Dashboard) writeCounts(b *strings.Builder) {
	fmt.Fprintf(b, "  Done:      %d\n", d.Done)
	fmt.Fprintf(b, "  Reviewing: %d\n", d.Reviewing)
	fmt.Fprintf(b, "  Rework:    %d\n", d.Rework)
	fmt.Fprintf(b, "  Pending:   %d\n", d.Pending)
	if d.Removed > 0 {
		fmt.Fprintf(b, "  Removed:   %d (retired from the plan, not counted)\n", d.Removed)
	}
}

// hasStages reports whether any slice carries a stage.
func (d *Dashboard) hasStages() bool {
	return len(d.ByStage) > 1 || (len(d.ByStage) == 1 && !hasKey(d.ByStage, "(none)"))
}

// writePhases lays the phases out as a table: the phase, its bar and
// counts, and its stages in plan order, the stages wrapped at stage
// boundaries to keep each line within width.
func (d *Dashboard) writePhases(b *strings.Builder, width int) {
	const gap = "  "
	counts := make([]string, len(d.PhaseOrder))
	nameW, countW := len("Phase"), 0
	for i, phase := range d.PhaseOrder {
		pp := d.ByPhase[phase]
		counts[i] = fmt.Sprintf("%d/%d (%.0f%%)", pp.Done, pp.Total, percent(pp.Done, pp.Total))
		nameW = max(nameW, utf8.RuneCountInString(phase))
		countW = max(countW, len(counts[i]))
	}
	progressW := phaseBarWidth + 2 + 1 + countW // [bar] counts
	indent := 2 + nameW + len(gap) + progressW + len(gap)
	stagesW := max(width-indent, minStagesWidth)

	b.WriteString("By Phase:\n")
	fmt.Fprintf(b, "  %-*s%s%-*s%sStages\n", nameW, "Phase", gap, progressW, "Progress", gap)
	for i, phase := range d.PhaseOrder {
		pp := d.ByPhase[phase]
		var stages []string
		for _, stage := range pp.StageOrder {
			if stage == "(none)" {
				continue
			}
			ps := pp.Stages[stage]
			stages = append(stages, fmt.Sprintf("%s %d/%d", stage, ps.Done, ps.Total))
		}
		lines := wrapItems(stages, stagesW)
		if len(lines) == 0 {
			lines = []string{"—"}
		}
		progress := progressBar(pp.Done, pp.Total, phaseBarWidth) + " " + fmt.Sprintf("%*s", countW, counts[i])
		fmt.Fprintf(b, "  %-*s%s%s%s%s\n", nameW, phase, gap, progress, gap, lines[0])
		for _, line := range lines[1:] {
			fmt.Fprintf(b, "%s%s\n", strings.Repeat(" ", indent), line)
		}
	}
}

const (
	// phaseBarWidth is the width of each phase's bar, inside the brackets.
	phaseBarWidth = 20
	// minStagesWidth keeps the stages column readable on a narrow terminal;
	// below it the line overflows rather than wrapping a word a line.
	minStagesWidth = 24
)

// wrapItems joins items with ", " into lines of at most width runes,
// breaking only between items; an item longer than width gets a line to
// itself. Every line but the last keeps its trailing comma.
func wrapItems(items []string, width int) []string {
	var lines []string
	line := ""
	for i, item := range items {
		if i < len(items)-1 {
			item += ","
		}
		switch {
		case line == "":
			line = item
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(item) <= width:
			line += " " + item
		default:
			lines = append(lines, line)
			line = item
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (d *Dashboard) writeStages(b *strings.Builder) {
	b.WriteString("By Stage (across phases, in the order the plans reached them):\n")
	for _, stage := range d.StageOrder {
		if stage == "(none)" {
			continue
		}
		sp := d.ByStage[stage]
		fmt.Fprintf(b, "  %-12s %d/%d (%.0f%%) %s\n",
			stage, sp.Done, sp.Total, percent(sp.Done, sp.Total), progressBar(sp.Done, sp.Total, 20))
	}
}

func percent(done, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(done) / float64(total) * 100
}

func progressBar(done, total, width int) string {
	if total == 0 {
		return "[" + strings.Repeat(" ", width) + "]"
	}
	filled := (done * width) / total
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
}

func hasKey(m map[string]StageProgress, key string) bool {
	_, ok := m[key]
	return ok
}
