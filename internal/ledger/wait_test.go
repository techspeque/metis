package ledger

import (
	"strings"
	"testing"
	"time"

	"github.com/techspeque/metis/internal/slice"
)

func waitLedger() *Ledger {
	return &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "first", Title: "first", Priority: slice.PriorityP1, Risk: slice.RiskMedium, Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
			{ID: "second", Title: "second", Priority: slice.PriorityP2, Risk: slice.RiskMedium, Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
		},
	}
}

func TestAWaitingSliceIsPassedOverUntilClearedOrExpired(t *testing.T) {
	l := waitLedger()
	now := time.Now()
	if err := l.Wait("first", "lab run crossing the hour", time.Time{}, now); err != nil {
		t.Fatal(err)
	}
	if got := l.Next().Slice.ID; got != "second" {
		t.Fatalf("dispatch while waiting: %s", got)
	}
	if w := l.WaitingSlices(); len(w) != 1 || w[0].ID != "first" || w[0].Waiting.Reason != "lab run crossing the hour" {
		t.Fatalf("waiting slices: %+v", w)
	}
	if err := l.ClearWait("first"); err != nil {
		t.Fatal(err)
	}
	if got := l.Next().Slice.ID; got != "first" {
		t.Fatalf("dispatch after clear: %s", got)
	}
	if err := l.ClearWait("first"); err == nil || !strings.Contains(err.Error(), "not waiting") {
		t.Fatalf("clearing twice: %v", err)
	}

	// A timed wait ends by itself.
	if err := l.Wait("first", "detector sweep", now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	s := l.FindByID("first")
	if !s.IsWaiting(now.Add(30*time.Minute)) || s.IsWaiting(now.Add(61*time.Minute)) {
		t.Fatal("a timed wait must hold until its instant and no longer")
	}
	if err := l.Wait("second", "x", now.Add(-time.Minute), now); err == nil || !strings.Contains(err.Error(), "not in the future") {
		t.Fatalf("a past until: %v", err)
	}
	if err := l.Wait("second", "", time.Time{}, now); err == nil || !strings.Contains(err.Error(), "reason") {
		t.Fatalf("a wait without a reason: %v", err)
	}
}

func TestAFlipClearsTheWait(t *testing.T) {
	l := waitLedger()
	if err := l.Wait("first", "approval", time.Time{}, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := l.FlipCoded("first"); err != nil {
		t.Fatal(err)
	}
	if l.FindByID("first").Waiting != nil {
		t.Fatal("flip coded must clear the wait")
	}
	if errs := l.Validate(nil, false); len(errs) != 0 {
		t.Fatalf("validate: %v", errs)
	}
	l.Slices[1].Waiting = &slice.Wait{Reason: "x", Since: "yesterday", Until: "soon"}
	errs := l.Validate(nil, false)
	if len(errs) != 2 {
		t.Fatalf("a malformed wait must fail validation: %v", errs)
	}
}

func TestASecondBlockJoinsTheCycle(t *testing.T) {
	l := waitLedger()
	if l.BlockedForReview("first") {
		t.Fatal("an uncoded slice is not blocked for review")
	}
	if err := l.FlipCoded("first"); err != nil {
		t.Fatal(err)
	}
	if err := l.Block("first"); err != nil {
		t.Fatal(err)
	}
	if !l.BlockedForReview("first") || l.FindByID("first").ReviewCycles != 1 {
		t.Fatal("after a block the slice is blocked for review in cycle 1")
	}
	if err := l.Block("first"); err == nil {
		t.Fatal("Block itself still refuses a second block; the CLI records the finding instead")
	}
}
