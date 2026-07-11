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
