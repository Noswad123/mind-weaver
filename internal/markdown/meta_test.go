package markdown

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureMetaID_ReplacesEmptyIDLine(t *testing.T) {
	content := "---\nid:\n---\n\n# Note\n"

	updated := EnsureMetaID(content, "note-id")
	id, ok := ReadMetaIDFromContent(updated)
	if !ok {
		t.Fatalf("ReadMetaIDFromContent() ok = false, want true; updated:\n%s", updated)
	}
	if id != "note-id" {
		t.Fatalf("ReadMetaIDFromContent() id = %q, want %q; updated:\n%s", id, "note-id", updated)
	}

	if strings.Count(updated, "id:") != 1 {
		t.Fatalf("updated frontmatter should contain one id field, got:\n%s", updated)
	}
}

func TestEnsureMetaID_InsertsIDIntoEmptyFrontmatterBlock(t *testing.T) {
	content := "---\n---\n\n# Note\n"

	updated := EnsureMetaID(content, "note-id")
	id, ok := ReadMetaIDFromContent(updated)
	if !ok {
		t.Fatalf("ReadMetaIDFromContent() ok = false, want true; updated:\n%s", updated)
	}
	if id != "note-id" {
		t.Fatalf("ReadMetaIDFromContent() id = %q, want %q; updated:\n%s", id, "note-id", updated)
	}
}

func TestEnsureMetaIDFile_WritesMissingID(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "note.md")

	if err := os.WriteFile(path, []byte("---\n---\n\n# Note\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	changed, err := EnsureMetaIDFile(path, "note")
	if err != nil {
		t.Fatalf("EnsureMetaIDFile() error = %v", err)
	}
	if !changed {
		t.Fatalf("EnsureMetaIDFile() changed = false, want true")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	id, ok := ReadMetaIDFromContent(string(b))
	if !ok {
		t.Fatalf("ReadMetaIDFromContent() ok = false, want true")
	}
	if id != "note" {
		t.Fatalf("ReadMetaIDFromContent() id = %q, want %q", id, "note")
	}
}
