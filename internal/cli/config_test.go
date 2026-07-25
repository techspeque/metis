package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigGetAndSet(t *testing.T) {
	dir := makeProject(t)
	t.Chdir(dir)
	setOutputFlag(t, "")

	if err := configSetCmd.RunE(configSetCmd, []string{"project.name", "renamed"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Read it back in JSON mode so the output is capturable.
	setOutputFlag(t, "json")
	var got string
	captureJSON(t, configGetCmd, []string{"project.name"}, &got)
	if got != "renamed" {
		t.Errorf("get after set = %q, want renamed", got)
	}
}

func TestConfigViewJSON(t *testing.T) {
	dir := makeProject(t)
	t.Chdir(dir)
	setOutputFlag(t, "json")

	var cfg struct {
		Version int `json:"version"`
		Paths   struct {
			Ledger string `json:"ledger"`
		} `json:"paths"`
	}
	captureJSON(t, configViewCmd, nil, &cfg)

	if cfg.Version != 1 {
		t.Errorf("view version = %d", cfg.Version)
	}
	if cfg.Paths.Ledger != ".metis/slices.yaml" {
		t.Errorf("view should show effective defaults, ledger = %q", cfg.Paths.Ledger)
	}
}

func TestConfigSetUnknownKey(t *testing.T) {
	dir := makeProject(t)
	t.Chdir(dir)

	err := configSetCmd.RunE(configSetCmd, []string{"projct.name", "typo"})
	if err == nil {
		t.Fatal("set with typo key: want error")
	}
	if !strings.Contains(err.Error(), "valid:") {
		t.Errorf("error should list valid keys: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "metis.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "projct") {
		t.Error("typo key must not be written to the file")
	}
}
