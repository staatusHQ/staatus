package render

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/staatusHQ/staatus/internal/config"
)

func TestRenderWritesPublicAPIFiles(t *testing.T) {
	cfg, err := config.Load(filepath.Join("..", "..", "staatus.yml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	out := filepath.Join(t.TempDir(), "api")
	manifest, err := Render(Options{
		Config:    cfg,
		OutputDir: out,
		DataDir:   filepath.Join("..", "..", "data"),
		Now:       time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if len(manifest.Files) != 3 {
		t.Fatalf("manifest files = %d, want 3", len(manifest.Files))
	}

	for _, name := range []string{"status.json", "components.json", "incidents.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
}
