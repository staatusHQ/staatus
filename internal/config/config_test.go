package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "staatus.yml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Page.Name != "Staatus Cloud" {
		t.Fatalf("page name = %q", cfg.Page.Name)
	}
	if cfg.CheckCount() != 3 {
		t.Fatalf("check count = %d", cfg.CheckCount())
	}
}

func TestLoadRejectsDuplicateComponentIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "staatus.yml")
	body := []byte(`page:
  name: Test
components:
  - id: api
    name: API
  - id: api
    name: API Copy
`)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
	if !IsValidationError(err) {
		t.Fatalf("Load() error = %T, want ValidationError", err)
	}
}
