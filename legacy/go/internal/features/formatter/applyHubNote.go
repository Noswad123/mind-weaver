package formatter

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
)

func applyHubUpdate(dirPath, notesRoot string, createMissingChildIssues bool) (bool, []string, error) {
	root := filepath.Clean(notesRoot)
	dir := filepath.Clean(dirPath)

	if err := validateTargetDir(dir, root); err != nil {
		return false, nil, err
	}

	hubPath := notefiles.PreferredHubNotePath(dir)
	if err := ensureHubFile(hubPath); err != nil {
		return false, nil, fmt.Errorf("ensure hub.md: %w", err)
	}

	snap, issues, err := buildSnapshot(dir, createMissingChildIssues)
	if err != nil {
		return false, nil, err
	}

	original, err := readFileString(hubPath)
	if err != nil {
		return false, nil, err
	}

	updated, changed, planIssues, err := planUpdate(original, snap)
	if err != nil {
		return false, nil, err
	}
	issues = append(issues, planIssues...)

	if !changed {
		return false, issues, nil
	}

	if err := os.WriteFile(hubPath, []byte(updated), 0o644); err != nil {
		return false, issues, fmt.Errorf("write %s: %w", hubPath, err)
	}

	return true, issues, nil
}

func validateTargetDir(dir, root string) error {
	if filepath.Base(dir) == ".git" {
		return fmt.Errorf("refusing to apply hub note rules inside .git: %s", dir)
	}
	if dir == root {
		return fmt.Errorf("refusing to apply hub note rules to notes root: %s", dir)
	}
	return nil
}

func readFileString(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
}

func ensureHubFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.WriteFile(path, []byte(defaultHubContent()), 0o644)
	}
	return nil
}
