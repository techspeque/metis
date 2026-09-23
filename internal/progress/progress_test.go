package progress

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/techspeque/metis/internal/slice"
)

func TestCompute_Empty(t *testing.T) {
	d := Compute(nil)
	if d.Total != 0 || d.Done != 0 {
		t.Errorf("empty: Total=%d, Done=%d", d.Total, d.Done)
	}
}

func TestCompute_Counts(t *testing.T) {
	slices := []slice.Slice{
		{Coded: true, Reviewed: true},                    // done
		{Coded: true, Reviewed: true},                    // done
		{Coded: true, Reviewed: false},                   // reviewing
		{Coded: false, Reviewed: false, ReviewCycles: 1}, // rework
		{Coded: false, Reviewed: false},                  // pending
	}

	d := Compute(slices)
	if d.Total != 5 {
		t.Errorf("Total = %d, want 5", d.Total)
	}
	if d.Done != 2 {
		t.Errorf("Done = %d, want 2", d.Done)
	}
	if d.Reviewing != 1 {
		t.Errorf("Reviewing = %d, want 1", d.Reviewing)
	}
	if d.Rework != 1 {
		t.Errorf("Rework = %d, want 1", d.Rework)
	}
	if d.Pending != 1 {
		t.Errorf("Pending = %d, want 1", d.Pending)
	}
}

func TestCompute_ByStage(t *testing.T) {
	slices := []slice.Slice{
		{Stage: "mvp", Coded: true, Reviewed: true},
		{Stage: "mvp", Coded: false, Reviewed: false},
		{Stage: "beta", Coded: false, Reviewed: false},
	}

	d := Compute(slices)
	if d.ByStage["mvp"].Total != 2 {
		t.Errorf("mvp total = %d, want 2", d.ByStage["mvp"].Total)
	}
	if d.ByStage["mvp"].Done != 1 {
		t.Errorf("mvp done = %d, want 1", d.ByStage["mvp"].Done)
	}
	if d.ByStage["beta"].Total != 1 {
		t.Errorf("beta total = %d, want 1", d.ByStage["beta"].Total)
	}
}

func TestDashboard_Render(t *testing.T) {
	slices := []slice.Slice{
		{Stage: "mvp", Coded: true, Reviewed: true},
		{Stage: "mvp", Coded: true, Reviewed: false},
		{Stage: "mvp", Coded: false, Reviewed: false},
	}

	d := Compute(slices)
	out := d.RenderView(ViewStats, 0)

	if !strings.Contains(out, "Metis Progress") {
		t.Error("render missing header")
	}
	if !strings.Contains(out, "1/3 done") {
		t.Error("render missing progress count")
	}
	if !strings.Contains(out, "33%") {
		t.Error("render missing percentage")
	}
	if !strings.Contains(out, "█") {
		t.Error("render missing progress bar")
	}
}

func TestDashboard_Render_Empty(t *testing.T) {
	d := Compute(nil)
	out := d.RenderView(ViewStats, 0)
	if !strings.Contains(out, "0/0 done") {
		t.Error("empty render missing 0/0")
	}
}

// TestCompute_RemovedNotCounted: retired slices left the plan — they must
// not inflate done, pending, or the completion denominator.
func TestCompute_RemovedNotCounted(t *testing.T) {
	d := Compute([]slice.Slice{
		{ID: "a", Coded: true, Reviewed: true},
		{ID: "b", Removed: true},
		{ID: "c"},
	})
	if d.Total != 2 || d.Done != 1 || d.Pending != 1 || d.Removed != 1 {
		t.Errorf("dashboard = %+v, want total 2, done 1, pending 1, removed 1", d)
	}
}

// TestRender_StagesInPlanOrder: the dashboard lists stages in the order
// the plan reached them (first appearance over archive then ledger), the
// same way on every run — never in map order.
func TestRender_StagesInPlanOrder(t *testing.T) {
	slices := []slice.Slice{
		{Stage: "foundation", Coded: true, Reviewed: true},
		{Stage: "contract", Coded: true, Reviewed: true},
		{Stage: "foundation", Coded: true, Reviewed: true},
		{Stage: "storage", Coded: true, Reviewed: true},
		{Stage: "adapters", Coded: false},
		{Stage: "contract", Coded: false},
		{Stage: "controls", Coded: false},
	}
	for run := 0; run < 5; run++ {
		d := Compute(slices)
		want := []string{"foundation", "contract", "storage", "adapters", "controls"}
		if strings.Join(d.StageOrder, ",") != strings.Join(want, ",") {
			t.Fatalf("StageOrder = %v, want %v", d.StageOrder, want)
		}
		out := d.RenderView(ViewStage, 0)
		last := -1
		for _, stage := range want {
			i := strings.Index(out, "  "+stage+" ")
			if i < 0 || i < last {
				t.Fatalf("stage %q rendered out of plan order in:\n%s", stage, out)
			}
			last = i
		}
	}
	// A stage-less slice contributes no row and no order entry beyond "(none)".
	d := Compute([]slice.Slice{{Coded: true, Reviewed: true}})
	out := d.RenderView(ViewStage, 0)
	if strings.Contains(out, "By Stage") || !strings.Contains(out, "No slices have a stage.") {
		t.Errorf("a single stage-less slice must not render a By Stage section:\n%s", out)
	}
}

// TestRender_PhasesInPlanOrder: phases sort by their number whatever the
// order the slices were archived in, an unplanned slice lands last, and
// each phase lists its own stages in the order the plan reached them —
// a stage name recurs across phases, so the flat stage view cannot say
// which phase is behind.
func TestRender_PhasesInPlanOrder(t *testing.T) {
	slices := []slice.Slice{
		{ID: "phase-10-ws-10.1", Plan: ".metis/plans/phase-10.md", Stage: "value", Coded: false},
		{ID: "phase-2-ws-2.1", Plan: ".metis/plans/phase-2.md", Stage: "foundation", Coded: true, Reviewed: true},
		{ID: "recon-0001", Stage: "", Coded: true, Reviewed: true},
		{ID: "phase-1-ws-1.1", Plan: ".metis/plans/phase-1.md", Stage: "substrate", Coded: true, Reviewed: true},
		{ID: "phase-1-ws-1.2", Plan: ".metis/plans/phase-1.md", Stage: "contract", Coded: true, Reviewed: true},
		{ID: "phase-1-gate", Plan: ".metis/plans/phase-1.md", Stage: "", Coded: true, Reviewed: true},
		{ID: "phase-6-ws-6.1", Stage: "substrate", Coded: true, Reviewed: true}, // no plan: the id names the phase
		{ID: "phase-6-ws-6.2", Plan: ".metis/plans/phase-6.md", Stage: "controls", Coded: false},
	}
	d := Compute(slices)
	want := []string{"phase-1", "phase-2", "phase-6", "phase-10", unplanned}
	if strings.Join(d.PhaseOrder, ",") != strings.Join(want, ",") {
		t.Fatalf("PhaseOrder = %v, want %v", d.PhaseOrder, want)
	}
	if p := d.ByPhase["phase-1"]; p.Total != 3 || p.Done != 3 || strings.Join(p.StageOrder, ",") != "substrate,contract,(none)" {
		t.Errorf("phase-1 = %+v", p)
	}
	if p := d.ByPhase["phase-6"]; p.Total != 2 || p.Done != 1 || p.Stages["substrate"].Done != 1 || p.Stages["controls"].Total != 1 {
		t.Errorf("phase-6 = %+v", p)
	}
	out := d.RenderView(ViewPhase, 0)
	last := -1
	for _, phase := range want {
		i := strings.Index(out, "  "+phase+" ")
		if i < 0 || i < last {
			t.Fatalf("phase %q rendered out of order in:\n%s", phase, out)
		}
		last = i
	}
	if !strings.Contains(out, "substrate 1/1, contract 1/1") {
		t.Errorf("phase-1's stages are not listed in plan order:\n%s", out)
	}
}

// TestRenderView_ShowsOnlyItsBreakdown: each view carries the overall line
// and its own section, never another view's.
func TestRenderView_ShowsOnlyItsBreakdown(t *testing.T) {
	d := Compute([]slice.Slice{
		{ID: "phase-1-ws-1.1", Plan: ".metis/plans/phase-1.md", Stage: "substrate", Coded: true, Reviewed: true},
		{ID: "phase-2-ws-2.1", Plan: ".metis/plans/phase-2.md", Stage: "contract", Coded: true},
		{ID: "phase-2-ws-2.2", Plan: ".metis/plans/phase-2.md", Stage: "contract", Removed: true},
	})
	cases := []struct {
		view    View
		want    []string
		notWant []string
	}{
		{ViewStats, []string{"Done:      1", "Reviewing: 1", "Removed:   1"}, []string{"By Phase", "By Stage"}},
		{ViewPhase, []string{"By Phase:", "phase-1 ", "phase-2 ", "substrate 1/1"}, []string{"Reviewing:", "By Stage"}},
		{ViewStage, []string{"By Stage", "substrate ", "contract "}, []string{"Reviewing:", "By Phase"}},
	}
	for _, c := range cases {
		out := d.RenderView(c.view, 0)
		if !strings.Contains(out, "Overall: 1/2 done (50%)") {
			t.Errorf("%s view is missing the overall line:\n%s", c.view, out)
		}
		for _, w := range c.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s view is missing %q:\n%s", c.view, w, out)
			}
		}
		for _, n := range c.notWant {
			if strings.Contains(out, n) {
				t.Errorf("%s view must not contain %q:\n%s", c.view, n, out)
			}
		}
	}
}

// TestRenderView_PhaseShowsUnplanned: asked for explicitly, the phase view
// shows a lone unplanned bucket rather than nothing; with no slices at all
// it says there is nothing to show.
func TestRenderView_PhaseShowsUnplanned(t *testing.T) {
	out := Compute([]slice.Slice{{ID: "recon-0001"}}).RenderView(ViewPhase, 0)
	if !strings.Contains(out, "  "+unplanned+" ") {
		t.Errorf("phase view should list the unplanned bucket:\n%s", out)
	}
	if out := Compute(nil).RenderView(ViewPhase, 0); !strings.Contains(out, "No phases to show.") {
		t.Errorf("empty phase view should say so:\n%s", out)
	}
}

func TestWrapItems(t *testing.T) {
	items := []string{"foundation 1/2", "contract 0/3", "storage 4/4"}
	cases := map[int][]string{
		100: {"foundation 1/2, contract 0/3, storage 4/4"},
		30:  {"foundation 1/2, contract 0/3,", "storage 4/4"},
		10:  {"foundation 1/2,", "contract 0/3,", "storage 4/4"},
	}
	for width, want := range cases {
		if got := wrapItems(items, width); strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("width %d: got %q, want %q", width, got, want)
		}
	}
	if got := wrapItems(nil, 10); got != nil {
		t.Errorf("no items: got %q", got)
	}
}

// TestRenderView_PhaseTable: the phase view is three columns — phase, bar
// with counts, stages — with every row's bar in the same column, the
// stages wrapped under the stages column, and no line past the width.
func TestRenderView_PhaseTable(t *testing.T) {
	var ss []slice.Slice
	for _, st := range []string{"foundation", "contract", "storage", "adapters", "controls", "observability"} {
		ss = append(ss, slice.Slice{ID: "phase-1-ws-" + st, Plan: ".metis/plans/phase-1.md", Stage: st, Coded: true, Reviewed: true})
	}
	ss = append(ss,
		slice.Slice{ID: "phase-12-ws-12.1", Plan: ".metis/plans/phase-12.md", Stage: "value"},
		slice.Slice{ID: "recon-0001", Coded: true, Reviewed: true},
	)
	const width = 80
	out := Compute(ss).RenderView(ViewPhase, width)
	_, table, found := strings.Cut(out, "By Phase:\n")
	if !found {
		t.Fatalf("no By Phase section:\n%s", out)
	}
	lines := strings.Split("By Phase:\n"+strings.TrimRight(table, "\n"), "\n")

	header := lines[1]
	barCol := runeIndex(header, "Progress")
	stageCol := runeIndex(header, "Stages")
	if barCol < 0 || stageCol < 0 || !strings.HasPrefix(header, "  Phase") {
		t.Fatalf("header = %q", header)
	}
	rows := map[string]string{}
	for _, line := range lines[2:] {
		if n := utf8.RuneCountInString(line); n > width {
			t.Errorf("line is %d wide, past %d: %q", n, width, line)
		}
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") {
			name := strings.Fields(line)[0]
			rows[name] = line
			if runeIndex(line, "[") != barCol {
				t.Errorf("%s: bar not under Progress:\n%s\n%s", name, header, line)
			}
			continue
		}
		// A continuation line carries only stages, starting under Stages.
		if strings.TrimLeft(line, " ") == "" || utf8.RuneCountInString(line)-utf8.RuneCountInString(strings.TrimLeft(line, " ")) != stageCol {
			t.Errorf("continuation not under Stages:\n%s\n%s", header, line)
		}
	}
	if continuations := len(lines) - 2 - len(rows); continuations == 0 {
		t.Errorf("phase-1's six stages should wrap at width %d:\n%s", width, out)
	}
	if runeIndex(rows["phase-1"], "foundation 1/1") != stageCol {
		t.Errorf("phase-1's stages should start under Stages:\n%s", out)
	}
	if !strings.HasSuffix(rows[unplanned], "—") {
		t.Errorf("a phase with no stages shows a dash: %q", rows[unplanned])
	}
	if !strings.Contains(rows["phase-12"], "  0/1 (0%)") || !strings.Contains(rows["phase-1"], "6/6 (100%)") {
		t.Errorf("counts should be right-aligned beside the bars:\n%s", out)
	}
}

func runeIndex(s, sub string) int {
	i := strings.Index(s, sub)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:i])
}
