package progress

import (
	"strings"
	"testing"

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
	out := d.Render()

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
	out := d.Render()
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
		out := d.Render()
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
	if strings.Contains(d.Render(), "By Stage") {
		t.Errorf("a single stage-less slice must not render a By Stage section:\n%s", d.Render())
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
	out := d.Render()
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
