package validation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Noswad123/mind-weaver/internal/core/registry"
)

func TestCollectFixIssues_FindsFilesystemDuplicates(t *testing.T) {
	root := t.TempDir()

	writeFixTestFile(t, filepath.Join(root, "a", "README.md"), "# A\n")
	writeFixTestFile(t, filepath.Join(root, "b", "README.md"), "# B\n")

	items, err := CollectFixIssues(context.Background(), root, nil, false)
	if err != nil {
		t.Fatalf("CollectFixIssues() error = %v", err)
	}

	var dupCount int
	for _, it := range items {
		if it.Reason == "DUPLICATE_ID" {
			dupCount++
		}
	}

	if dupCount != 2 {
		t.Fatalf("DUPLICATE_ID count = %d, want 2", dupCount)
	}
}

func TestCollectFixIssues_RegistryWarnFiltering(t *testing.T) {
	uidWarn := "new-note"
	uidErr := "dup"
	reader := fakeFixRegistryReader{
		conflicts: []registry.Conflict{
			{Path: "notes/a.md", Reason: "NOTE_NOT_IN_DB", UID: &uidWarn},
			{Path: "notes/b.md", Reason: "DUPLICATE_ID", UID: &uidErr},
		},
	}

	itemsErrorsOnly, err := CollectFixIssues(context.Background(), t.TempDir(), reader, false)
	if err != nil {
		t.Fatalf("CollectFixIssues(includeWarn=false) error = %v", err)
	}

	for _, it := range itemsErrorsOnly {
		if it.Reason == "NOTE_NOT_IN_DB" {
			t.Fatalf("unexpected NOTE_NOT_IN_DB when includeWarn=false")
		}
	}

	itemsAll, err := CollectFixIssues(context.Background(), t.TempDir(), reader, true)
	if err != nil {
		t.Fatalf("CollectFixIssues(includeWarn=true) error = %v", err)
	}

	var hasWarn bool
	for _, it := range itemsAll {
		if it.Reason == "NOTE_NOT_IN_DB" {
			hasWarn = true
			break
		}
	}
	if !hasWarn {
		t.Fatalf("expected NOTE_NOT_IN_DB when includeWarn=true")
	}
}

func TestFixCache_WriteAndLoad(t *testing.T) {
	root := t.TempDir()
	payload := FixCache{
		SchemaVersion: fixCacheSchemaVersion,
		GeneratedAt:   "2026-04-18T00:00:00Z",
		NotesDir:      root,
		Items: []FixIssue{
			{Path: "a/README.md", UID: "README", Reason: "DUPLICATE_ID", Severity: "ERROR", Source: "filesystem"},
		},
	}

	cachePath, err := writeFixCache(root, payload)
	if err != nil {
		t.Fatalf("writeFixCache() error = %v", err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("cache file missing: %v", err)
	}

	got, err := loadFixCache(root)
	if err != nil {
		t.Fatalf("loadFixCache() error = %v", err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("loaded items len = %d, want 1", len(got.Items))
	}
	if got.Items[0].Reason != "DUPLICATE_ID" {
		t.Fatalf("loaded reason = %q, want DUPLICATE_ID", got.Items[0].Reason)
	}
}

type fakeFixRegistryReader struct {
	conflicts []registry.Conflict
}

func (f fakeFixRegistryReader) ListEntries(context.Context) ([]registry.Entry, error) {
	return nil, nil
}

func (f fakeFixRegistryReader) ListConflicts(context.Context) ([]registry.Conflict, error) {
	return f.conflicts, nil
}

func writeFixTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
