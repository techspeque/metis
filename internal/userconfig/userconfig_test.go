package userconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempConfig points METIS_USER_CONFIG at a temp file and returns its path.
func withTempConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv(EnvPath, path)
	return path
}

func TestPathEnvOverride(t *testing.T) {
	t.Setenv(EnvPath, "/custom/path/config.yaml")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if got != "/custom/path/config.yaml" {
		t.Errorf("Path() = %q, want env override", got)
	}
}

func TestPathDefault(t *testing.T) {
	t.Setenv(EnvPath, "")
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".metis", "config.yaml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestLoadMissingFile(t *testing.T) {
	withTempConfig(t)
	uc, err := Load()
	if err != nil {
		t.Fatalf("Load() on missing file: %v", err)
	}
	if uc.Active != "" || len(uc.Workspaces) != 0 {
		t.Errorf("Load() on missing file = %+v, want empty config", uc)
	}
}

func TestLoadMalformed(t *testing.T) {
	path := withTempConfig(t)
	if err := os.WriteFile(path, []byte("workspaces: [not: a: map"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load()
	if err == nil {
		t.Fatal("Load() on malformed file: want error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q should name the config path", err)
	}
}

func TestRoundTrip(t *testing.T) {
	path := withTempConfig(t)

	uc := &UserConfig{Workspaces: map[string]string{}}
	if err := uc.Add("metis", "/home/u/proj/metis"); err != nil {
		t.Fatal(err)
	}
	if err := uc.Add("acme", "/home/u/proj/acme"); err != nil {
		t.Fatal(err)
	}
	if err := uc.Use("metis"); err != nil {
		t.Fatal(err)
	}
	if err := uc.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "# ~/.metis/config.yaml") {
		t.Errorf("saved file missing header comment:\n%s", data)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Active != "metis" {
		t.Errorf("Active = %q, want metis", loaded.Active)
	}
	if loaded.Workspaces["acme"] != "/home/u/proj/acme" {
		t.Errorf("Workspaces = %v", loaded.Workspaces)
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.yaml")
	t.Setenv(EnvPath, path)

	uc := &UserConfig{Workspaces: map[string]string{"a": "/x"}}
	if err := uc.Save(); err != nil {
		t.Fatalf("Save() should create parent directories: %v", err)
	}
}

func TestAddValidation(t *testing.T) {
	uc := &UserConfig{Workspaces: map[string]string{}}

	if err := uc.Add("", "/abs"); err == nil {
		t.Error("Add with empty name: want error")
	}
	if err := uc.Add("rel", "relative/path"); err == nil {
		t.Error("Add with relative path: want error")
	}
	if err := uc.Add("a", "/p1"); err != nil {
		t.Fatal(err)
	}
	if err := uc.Add("a", "/p2"); err == nil {
		t.Error("Add duplicate name with different path: want error")
	}
	// Re-adding the same name+path is idempotent.
	if err := uc.Add("a", "/p1"); err != nil {
		t.Errorf("Add same name+path should be idempotent: %v", err)
	}
}

func TestRemove(t *testing.T) {
	uc := &UserConfig{
		Active:     "a",
		Workspaces: map[string]string{"a": "/p1", "b": "/p2"},
	}

	if err := uc.Remove("missing"); err == nil {
		t.Error("Remove unknown: want error")
	}
	if err := uc.Remove("a"); err != nil {
		t.Fatal(err)
	}
	if uc.Active != "" {
		t.Errorf("removing active workspace should clear Active, got %q", uc.Active)
	}
	if err := uc.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if len(uc.Workspaces) != 0 {
		t.Errorf("Workspaces = %v, want empty", uc.Workspaces)
	}
}

func TestUse(t *testing.T) {
	uc := &UserConfig{Workspaces: map[string]string{"a": "/p1"}}

	if err := uc.Use("missing"); err == nil {
		t.Error("Use unknown: want error")
	}
	if err := uc.Use("a"); err != nil {
		t.Fatal(err)
	}
	if uc.Active != "a" {
		t.Errorf("Active = %q, want a", uc.Active)
	}
}

func TestNamesSorted(t *testing.T) {
	uc := &UserConfig{Workspaces: map[string]string{"c": "/1", "a": "/2", "b": "/3"}}
	names := uc.Names()
	want := []string{"a", "b", "c"}
	for i, n := range names {
		if n != want[i] {
			t.Fatalf("Names() = %v, want %v", names, want)
		}
	}
}
