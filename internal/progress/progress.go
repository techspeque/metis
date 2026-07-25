// Package progress renders the terminal progress dashboard showing
// per-phase progress bars, completion percentages, and quality stats.
package progress

import (
	"fmt"
	"strings"

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
	Active    *slice.Slice             `json:"active,omitempty"`
	ByStage   map[string]StageProgress `json:"by_stage,omitempty"`
}

// StageProgress holds progress for a single stage.
type StageProgress struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

// Compute builds a Dashboard from a slice list.
func Compute(slices []slice.Slice) *Dashboard {
	d := &Dashboard{
		Total:   len(slices),
		ByStage: make(map[string]StageProgress),
	}

	for i := range slices {
		stage := slices[i].Stage
		if stage == "" {
			stage = "(none)"
		}
		sp := d.ByStage[stage]
		sp.Total++

		switch slices[i].Status() {
		case slice.StatusDone:
			d.Done++
			sp.Done++
		case slice.StatusReviewing:
			d.Reviewing++
		case slice.StatusRework:
			d.Rework++
		default:
			d.Pending++
		}

		d.ByStage[stage] = sp
	}

	return d
}

// Render produces the text dashboard output.
func (d *Dashboard) Render() string {
	var b strings.Builder

	pct := 0.0
	if d.Total > 0 {
		pct = float64(d.Done) / float64(d.Total) * 100
	}

	b.WriteString("═══ Metis Progress ═══\n\n")
	fmt.Fprintf(&b, "Overall: %d/%d done (%.0f%%)\n", d.Done, d.Total, pct)
	b.WriteString(progressBar(d.Done, d.Total, 40))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "  Done:      %d\n", d.Done)
	fmt.Fprintf(&b, "  Reviewing: %d\n", d.Reviewing)
	fmt.Fprintf(&b, "  Rework:    %d\n", d.Rework)
	fmt.Fprintf(&b, "  Pending:   %d\n", d.Pending)

	if len(d.ByStage) > 1 || (len(d.ByStage) == 1 && !hasKey(d.ByStage, "(none)")) {
		b.WriteString("\nBy Stage:\n")
		for stage, sp := range d.ByStage {
			if stage == "(none)" {
				continue
			}
			sPct := 0.0
			if sp.Total > 0 {
				sPct = float64(sp.Done) / float64(sp.Total) * 100
			}
			fmt.Fprintf(&b, "  %-12s %d/%d (%.0f%%) %s\n",
				stage, sp.Done, sp.Total, sPct, progressBar(sp.Done, sp.Total, 20))
		}
	}

	return b.String()
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
