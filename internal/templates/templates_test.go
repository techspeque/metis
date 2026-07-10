package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAll_CreatesFiles(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "templates")

	if err := WriteAll(dir); err != nil {
		t.Fatalf("WriteAll() error: %v", err)
	}

	expected := []string{"plan.md", "overview.md", "adr.md", "recon.md", "gate.md"}
	for _, name := range expected {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			t.Errorf("WriteAll() did not create %s", name)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", name)
		}
	}
}

func TestWriteAll_Idempotent(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "templates")

	// Write twice — should not error
	if err := WriteAll(dir); err != nil {
		t.Fatalf("first WriteAll() error: %v", err)
	}
	if err := WriteAll(dir); err != nil {
		t.Fatalf("second WriteAll() error: %v", err)
	}
}

func TestPlanTemplate_NotEmpty(t *testing.T) {
	if len(PlanTemplate) < 100 {
		t.Error("PlanTemplate seems too short")
	}
}

func TestOverviewTemplate_NotEmpty(t *testing.T) {
	if len(OverviewTemplate) < 100 {
		t.Error("OverviewTemplate seems too short")
	}
}

func TestADRTemplate_NotEmpty(t *testing.T) {
	if len(ADRTemplate) < 100 {
		t.Error("ADRTemplate seems too short")
	}
}

func TestReconTemplate_NotEmpty(t *testing.T) {
	if len(ReconTemplate) < 100 {
		t.Error("ReconTemplate seems too short")
	}
}

func TestGateTemplate_NotEmpty(t *testing.T) {
	if len(GateTemplate) < 100 {
		t.Error("GateTemplate seems too short")
	}
}
