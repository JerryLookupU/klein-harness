package instructions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverOrdersGlobalThenProjectThenNested(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	nested := filepath.Join(root, "services", "payments")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	for _, path := range []string{
		filepath.Join(home, "AGENTS.md"),
		filepath.Join(root, ".git"),
		filepath.Join(root, "AGENTS.md"),
		filepath.Join(nested, "AGENTS.override.md"),
	} {
		if err := os.WriteFile(path, []byte(path), 0o644); err != nil {
			t.Fatalf("write file %s: %v", path, err)
		}
	}
	files, err := Discover(nested, home)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("unexpected file count: %+v", files)
	}
	if files[0].Scope != "global" || files[1].Scope != "project" || files[2].Scope != "nested" {
		t.Fatalf("unexpected order/scopes: %+v", files)
	}
}
