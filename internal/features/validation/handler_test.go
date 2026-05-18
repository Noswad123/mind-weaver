package validation

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/Noswad123/mind-weaver/internal/core/registry"
	"github.com/urfave/cli/v2"
)

func TestValidateFilesystem_FailsOnDuplicateID(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "a", "README.md"), "# A\n")
	writeTestFile(t, filepath.Join(root, "b", "README.md"), "# B\n")

	hasError, err := validateFilesystem(root)
	if err != nil {
		t.Fatalf("validateFilesystem() error = %v", err)
	}
	if !hasError {
		t.Fatalf("validateFilesystem() hasError = false, want true")
	}
}

func TestValidateFilesystem_PassesWithoutConflicts(t *testing.T) {
	root := t.TempDir()

	writeTestFile(t, filepath.Join(root, "one.md"), "# One\n")
	writeTestFile(t, filepath.Join(root, "two.md"), "# Two\n")

	hasError, err := validateFilesystem(root)
	if err != nil {
		t.Fatalf("validateFilesystem() error = %v", err)
	}
	if hasError {
		t.Fatalf("validateFilesystem() hasError = true, want false")
	}
}

func TestRunRegistry_FailsOnDuplicateConflict(t *testing.T) {
	uid := "README"
	reader := fakeRegistryReader{
		conflicts: []registry.Conflict{
			{UID: &uid, Path: "a/README.md", Reason: "DUPLICATE_ID"},
		},
	}

	err := RunRegistry(newCLIContext(t), t.TempDir(), reader)
	if err == nil {
		t.Fatalf("RunRegistry() error = nil, want non-nil")
	}
}

func TestRunRegistry_PassesOnWarnOnlyConflict(t *testing.T) {
	uid := "new-note"
	reader := fakeRegistryReader{
		conflicts: []registry.Conflict{
			{UID: &uid, Path: "x/new-note.md", Reason: "NOTE_NOT_IN_DB"},
		},
	}

	err := RunRegistry(newCLIContext(t), t.TempDir(), reader)
	if err != nil {
		t.Fatalf("RunRegistry() error = %v, want nil", err)
	}
}

type fakeRegistryReader struct {
	conflicts []registry.Conflict
}

func (f fakeRegistryReader) ListEntries(context.Context) ([]registry.Entry, error) {
	return nil, nil
}

func (f fakeRegistryReader) ListConflicts(context.Context) ([]registry.Conflict, error) {
	return f.conflicts, nil
}

func newCLIContext(t *testing.T) *cli.Context {
	t.Helper()

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	return cli.NewContext(cli.NewApp(), fs, nil)
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
