package registration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuild_DerivesHubIDFromParentFolderWhenMissing(t *testing.T) {
	root := t.TempDir()
	hubPath := filepath.Join(root, "projects", "hub.md")
	writeTestFile(t, hubPath, "# Projects\n")

	res, err := Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entry, ok := res.Registry.Entries["projects"]
	if !ok {
		t.Fatalf("expected registry entry for derived hub id %q", "projects")
	}
	if entry.Path != "projects/hub.md" {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, "projects/hub.md")
	}
	if len(res.MissingHub) != 0 {
		t.Fatalf("MissingHub = %v, want empty", res.MissingHub)
	}
}

func TestBuild_DerivesRootHubIDFromNotesRootFolder(t *testing.T) {
	root := t.TempDir()
	rootBase := filepath.Base(root)
	hubPath := filepath.Join(root, "hub.md")
	writeTestFile(t, hubPath, "# Root Hub\n")

	res, err := Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	entry, ok := res.Registry.Entries[rootBase]
	if !ok {
		t.Fatalf("expected registry entry for derived root hub id %q", rootBase)
	}
	if entry.Path != "hub.md" {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, "hub.md")
	}
}

func TestBuild_PreservesExplicitHubID(t *testing.T) {
	root := t.TempDir()
	hubPath := filepath.Join(root, "projects", "hub.md")
	writeTestFile(t, hubPath, "---\nid: custom-id\n---\n\n# Projects\n")

	res, err := Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if _, ok := res.Registry.Entries["projects"]; ok {
		t.Fatalf("did not expect derived id when explicit id exists")
	}

	entry, ok := res.Registry.Entries["custom-id"]
	if !ok {
		t.Fatalf("expected registry entry for explicit id %q", "custom-id")
	}
	if entry.Path != "projects/hub.md" {
		t.Fatalf("entry.Path = %q, want %q", entry.Path, "projects/hub.md")
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
