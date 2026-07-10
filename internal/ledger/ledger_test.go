package ledger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/techspeque/metis/internal/slice"
)

func TestLoad_NonExistent(t *testing.T) {
	l, err := Load("/nonexistent/path/slices.yaml")
	if err != nil {
		t.Fatalf("Load non-existent should return empty ledger, got error: %v", err)
	}
	if len(l.Slices) != 0 {
		t.Errorf("expected empty slices, got %d", len(l.Slices))
	}
}

func TestLoad_FromTestdata(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "slices.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("testdata/slices.yaml not found")
	}

	l, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(l.Slices) != 5 {
		t.Errorf("expected 5 slices, got %d", len(l.Slices))
	}
	if l.Slices[0].ID != "phase-0-ws-0.1" {
		t.Errorf("first slice ID = %q", l.Slices[0].ID)
	}
}

func TestLedger_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".metis", "slices.yaml")

	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{
				ID: "feat-0001", Title: "Test", Type: slice.TypeFeat,
				Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Coder: "agent-a", Reviewer: "agent-b", Created: "2026-07-10",
			},
		},
	}

	if err := l.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Slices) != 1 {
		t.Fatalf("expected 1 slice, got %d", len(loaded.Slices))
	}
	if loaded.Slices[0].ID != "feat-0001" {
		t.Errorf("slice ID = %q, want %q", loaded.Slices[0].ID, "feat-0001")
	}
}

func TestLedger_FindByID(t *testing.T) {
	l := sampleLedger()

	s := l.FindByID("feat-0002")
	if s == nil {
		t.Fatal("FindByID returned nil")
	}
	if s.Title != "Second" {
		t.Errorf("Title = %q, want %q", s.Title, "Second")
	}

	if l.FindByID("nonexistent") != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestLedger_Add(t *testing.T) {
	l := sampleLedger()
	initial := len(l.Slices)

	s := slice.Slice{ID: "feat-0099", Title: "New"}
	if err := l.Add(s); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if len(l.Slices) != initial+1 {
		t.Errorf("expected %d slices, got %d", initial+1, len(l.Slices))
	}

	// Duplicate should fail
	if err := l.Add(s); err == nil {
		t.Error("expected error for duplicate ID")
	}
}

func TestLedger_AddAfter(t *testing.T) {
	l := sampleLedger()
	s := slice.Slice{ID: "inserted", Title: "Inserted"}

	if err := l.AddAfter(s, "feat-0001"); err != nil {
		t.Fatalf("AddAfter() error: %v", err)
	}
	if l.Slices[1].ID != "inserted" {
		t.Errorf("expected inserted at index 1, got %q", l.Slices[1].ID)
	}
}

func TestLedger_AddBefore(t *testing.T) {
	l := sampleLedger()
	s := slice.Slice{ID: "inserted", Title: "Inserted"}

	if err := l.AddBefore(s, "feat-0002"); err != nil {
		t.Fatalf("AddBefore() error: %v", err)
	}
	if l.Slices[1].ID != "inserted" {
		t.Errorf("expected inserted at index 1, got %q", l.Slices[1].ID)
	}
	if l.Slices[2].ID != "feat-0002" {
		t.Errorf("expected feat-0002 at index 2, got %q", l.Slices[2].ID)
	}
}

func TestNext_PriorityOrdering(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "normal", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
			{ID: "urgent", Priority: slice.PriorityP0, Risk: slice.RiskHigh,
				Type: slice.TypeSecurity, Coder: "a", Reviewer: "b"},
			{ID: "low", Priority: slice.PriorityP3, Risk: slice.RiskLow,
				Type: slice.TypeChore, Coder: "a", Reviewer: "b"},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	if result.Slice.ID != "urgent" {
		t.Errorf("Next() returned %q, want %q (p0 should win)", result.Slice.ID, "urgent")
	}
	if result.Role != slice.RoleCoder {
		t.Errorf("Role = %q, want Coder", result.Role)
	}
}

func TestNext_DeclarationOrder(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "first", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
			{ID: "second", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	if result.Slice.ID != "first" {
		t.Errorf("Next() = %q, want %q (declaration order)", result.Slice.ID, "first")
	}
}

func TestNext_SkipsDone(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "done", Priority: slice.PriorityP2, Coded: true, Reviewed: true,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
			{ID: "pending", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	if result.Slice.ID != "pending" {
		t.Errorf("Next() = %q, want %q", result.Slice.ID, "pending")
	}
}

func TestNext_ReviewerRole(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "needs-review", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "coder-agent", Reviewer: "reviewer-agent",
				Coded: true, Reviewed: false},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	if result.Role != slice.RoleReviewer {
		t.Errorf("Role = %q, want Reviewer", result.Role)
	}
	if result.AgentSlug != "reviewer-agent" {
		t.Errorf("AgentSlug = %q, want %q", result.AgentSlug, "reviewer-agent")
	}
}

func TestNext_BlockedBy(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "dep", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b",
				Coded: false},
			{ID: "blocked", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b",
				BlockedBy: []string{"dep"}},
			{ID: "unblocked", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b"},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	// "dep" comes first in declaration order and is unblocked
	if result.Slice.ID != "dep" {
		t.Errorf("Next() = %q, want %q", result.Slice.ID, "dep")
	}
}

func TestNext_BlockedByDone(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "dep", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b",
				Coded: true, Reviewed: true},
			{ID: "was-blocked", Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Type: slice.TypeFeat, Coder: "a", Reviewer: "b",
				BlockedBy: []string{"dep"}},
		},
	}

	result := l.Next()
	if result == nil {
		t.Fatal("Next() returned nil")
	}
	// "dep" is done, so "was-blocked" is now unblocked
	if result.Slice.ID != "was-blocked" {
		t.Errorf("Next() = %q, want %q", result.Slice.ID, "was-blocked")
	}
}

func TestNext_Empty(t *testing.T) {
	l := &Ledger{Version: 1, Slices: []slice.Slice{}}
	if l.Next() != nil {
		t.Error("expected nil for empty ledger")
	}
}

func TestNext_AllDone(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "done", Coded: true, Reviewed: true, Coder: "a", Reviewer: "b"},
		},
	}
	if l.Next() != nil {
		t.Error("expected nil when all slices are done")
	}
}

func TestFlipCoded(t *testing.T) {
	l := sampleLedger()
	if err := l.FlipCoded("feat-0001"); err != nil {
		t.Fatalf("FlipCoded() error: %v", err)
	}
	if !l.FindByID("feat-0001").Coded {
		t.Error("expected coded=true after flip")
	}

	// Double flip should fail
	if err := l.FlipCoded("feat-0001"); err == nil {
		t.Error("expected error for already-coded slice")
	}

	// Non-existent
	if err := l.FlipCoded("nonexistent"); err == nil {
		t.Error("expected error for non-existent slice")
	}
}

func TestFlipReviewed(t *testing.T) {
	l := sampleLedger()
	l.FindByID("feat-0001").Coded = true

	if err := l.FlipReviewed("feat-0001", "agent-b"); err != nil {
		t.Fatalf("FlipReviewed() error: %v", err)
	}
	if !l.FindByID("feat-0001").Reviewed {
		t.Error("expected reviewed=true after flip")
	}

	// Cannot review uncoded slice
	if err := l.FlipReviewed("feat-0002", "agent-b"); err == nil {
		t.Error("expected error for uncoded slice")
	}

	// Cannot self-review
	l.FindByID("feat-0002").Coded = true
	if err := l.FlipReviewed("feat-0002", "agent-a"); err == nil {
		t.Error("expected error for self-review (reviewer == coder)")
	}
}

func TestBlock(t *testing.T) {
	l := sampleLedger()
	l.FindByID("feat-0001").Coded = true

	if err := l.Block("feat-0001"); err != nil {
		t.Fatalf("Block() error: %v", err)
	}
	s := l.FindByID("feat-0001")
	if s.Coded {
		t.Error("expected coded=false after block")
	}
	if s.ReviewCycles != 1 {
		t.Errorf("ReviewCycles = %d, want 1", s.ReviewCycles)
	}
}

func TestSkip(t *testing.T) {
	l := sampleLedger()
	if err := l.Skip("feat-0001", "not needed"); err != nil {
		t.Fatalf("Skip() error: %v", err)
	}
	s := l.FindByID("feat-0001")
	if !s.IsDone() {
		t.Error("expected done after skip")
	}
	if s.Notes == "" {
		t.Error("expected notes to contain skip reason")
	}
}

func TestReopen(t *testing.T) {
	l := sampleLedger()
	l.FindByID("feat-0001").Coded = true
	l.FindByID("feat-0001").Reviewed = true

	if err := l.Reopen("feat-0001", "requirements changed"); err != nil {
		t.Fatalf("Reopen() error: %v", err)
	}
	s := l.FindByID("feat-0001")
	if s.Coded || s.Reviewed {
		t.Error("expected coded=false, reviewed=false after reopen")
	}
}

func TestArchive(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "done-1", Coded: true, Reviewed: true, Coder: "a", Reviewer: "b"},
			{ID: "pending", Coded: false, Reviewed: false, Coder: "a", Reviewer: "b"},
			{ID: "done-2", Coded: true, Reviewed: true, Coder: "a", Reviewer: "b"},
		},
	}
	archive := &Archive{Version: 1, Slices: []slice.Slice{}}

	archived := l.Archive(archive)
	if len(archived) != 2 {
		t.Errorf("archived %d, want 2", len(archived))
	}
	if len(l.Slices) != 1 {
		t.Errorf("remaining %d, want 1", len(l.Slices))
	}
	if l.Slices[0].ID != "pending" {
		t.Errorf("remaining slice = %q, want %q", l.Slices[0].ID, "pending")
	}
	if len(archive.Slices) != 2 {
		t.Errorf("archive has %d, want 2", len(archive.Slices))
	}
}

func TestValidate_Valid(t *testing.T) {
	l := sampleLedger()
	errs := l.Validate(nil)
	if len(errs) != 0 {
		t.Errorf("valid ledger has errors: %v", errs)
	}
}

func TestValidate_DuplicateIDs(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "dup", Title: "A", Type: slice.TypeFeat, Priority: slice.PriorityP2,
				Risk: slice.RiskMedium, Coder: "a", Reviewer: "b"},
			{ID: "dup", Title: "B", Type: slice.TypeFeat, Priority: slice.PriorityP2,
				Risk: slice.RiskMedium, Coder: "a", Reviewer: "b"},
		},
	}
	errs := l.Validate(nil)
	if len(errs) == 0 {
		t.Error("expected error for duplicate IDs")
	}
}

func TestValidate_AgentSlugs(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "feat-0001", Title: "A", Type: slice.TypeFeat, Priority: slice.PriorityP2,
				Risk: slice.RiskMedium, Coder: "unknown-agent", Reviewer: "b"},
		},
	}
	agents := map[string]bool{"b": true}
	errs := l.Validate(agents)
	found := false
	for _, e := range errs {
		if e.Error() == "feat-0001: coder \"unknown-agent\" not in agents map" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected agent slug error, got: %v", errs)
	}
}

func TestValidate_CircularDeps(t *testing.T) {
	l := &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{ID: "a", Title: "A", Type: slice.TypeFeat, Priority: slice.PriorityP2,
				Risk: slice.RiskMedium, Coder: "x", Reviewer: "y", BlockedBy: []string{"b"}},
			{ID: "b", Title: "B", Type: slice.TypeFeat, Priority: slice.PriorityP2,
				Risk: slice.RiskMedium, Coder: "x", Reviewer: "y", BlockedBy: []string{"a"}},
		},
	}
	errs := l.Validate(nil)
	found := false
	for _, e := range errs {
		if e != nil {
			found = true
		}
	}
	if !found {
		t.Error("expected circular dependency error")
	}
}

// sampleLedger returns a ledger for testing.
func sampleLedger() *Ledger {
	return &Ledger{
		Version: 1,
		Slices: []slice.Slice{
			{
				ID: "feat-0001", Title: "First", Type: slice.TypeFeat,
				Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Coder: "agent-a", Reviewer: "agent-b", Created: "2026-07-01",
			},
			{
				ID: "feat-0002", Title: "Second", Type: slice.TypeFeat,
				Priority: slice.PriorityP2, Risk: slice.RiskMedium,
				Coder: "agent-a", Reviewer: "agent-b", Created: "2026-07-02",
			},
		},
	}
}
