package findings

import (
	"path/filepath"
	"testing"
)

func TestStore_AddAndFilter(t *testing.T) {
	s := &Store{Findings: []Finding{}}

	id1 := s.Add("feat-0001", "P1", "auth", "Missing auth check")
	id2 := s.Add("feat-0002", "P2", "scope", "Edited out-of-scope file")
	id3 := s.Add("feat-0001", "P2", "tests", "No regression test")

	if id1 != "f-001" || id2 != "f-002" || id3 != "f-003" {
		t.Errorf("IDs = %s, %s, %s; want f-001, f-002, f-003", id1, id2, id3)
	}

	if len(s.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(s.Findings))
	}

	// Filter by severity
	p1 := s.Filter("P1", "", "")
	if len(p1) != 1 {
		t.Errorf("P1 filter: got %d, want 1", len(p1))
	}

	// Filter by category
	auth := s.Filter("", "auth", "")
	if len(auth) != 1 {
		t.Errorf("auth filter: got %d, want 1", len(auth))
	}

	// Filter by slice
	slice1 := s.Filter("", "", "feat-0001")
	if len(slice1) != 2 {
		t.Errorf("feat-0001 filter: got %d, want 2", len(slice1))
	}

	// Combined filter
	combined := s.Filter("P2", "", "feat-0001")
	if len(combined) != 1 {
		t.Errorf("combined filter: got %d, want 1", len(combined))
	}
}

func TestStore_FindByID(t *testing.T) {
	s := &Store{Findings: []Finding{}}
	s.Add("feat-0001", "P1", "auth", "Finding one")
	s.Add("feat-0002", "P2", "scope", "Finding two")

	f := s.FindByID("f-001")
	if f == nil {
		t.Fatal("FindByID returned nil")
		return
	}
	if f.Finding != "Finding one" {
		t.Errorf("Finding = %q", f.Finding)
	}

	if s.FindByID("nonexistent") != nil {
		t.Error("expected nil for nonexistent ID")
	}
}

func TestStore_GetStats(t *testing.T) {
	s := &Store{Findings: []Finding{}}
	s.Add("feat-0001", "P1", "auth", "a")
	s.Add("feat-0002", "P1", "auth", "b")
	s.Add("feat-0003", "P2", "scope", "c")

	stats := s.GetStats()
	if stats.Total != 3 {
		t.Errorf("Total = %d, want 3", stats.Total)
	}
	if stats.BySeverity["P1"] != 2 {
		t.Errorf("P1 count = %d, want 2", stats.BySeverity["P1"])
	}
	if stats.ByCategory["auth"] != 2 {
		t.Errorf("auth count = %d, want 2", stats.ByCategory["auth"])
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "findings.yaml")

	s := &Store{Findings: []Finding{}}
	s.Add("feat-0001", "P1", "auth", "Test finding")

	if err := s.Save(path); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded.Findings) != 1 {
		t.Fatalf("loaded %d findings, want 1", len(loaded.Findings))
	}
	if loaded.Findings[0].Finding != "Test finding" {
		t.Errorf("Finding = %q", loaded.Findings[0].Finding)
	}
}

func TestLoad_NonExistent(t *testing.T) {
	s, err := Load("/nonexistent/findings.yaml")
	if err != nil {
		t.Fatalf("Load non-existent should return empty store, got error: %v", err)
	}
	if len(s.Findings) != 0 {
		t.Errorf("expected empty findings, got %d", len(s.Findings))
	}
}
