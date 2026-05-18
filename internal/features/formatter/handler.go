package formatter

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
	mdregistry "github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/Noswad123/mind-weaver/internal/markdown"
	"github.com/urfave/cli/v2"
)

func Run(c *cli.Context, notesDir string) error {
	return run(notesDir, c.Bool("all"))
}

func RunAll(notesDir string) error {
	return run(notesDir, true)
}

func run(notesDir string, formatAllNotes bool) error {
	root := filepath.Clean(notesDir)

	logStart(formatAllNotes)

	// Pre-scan: refuse to run if duplicates exist
	reg, err := mdregistry.Build(root)
	if err != nil {
		return cli.Exit("❌ Registry scan failed: "+err.Error(), 1)
	}
	if len(reg.Duplicates) > 0 {
		return cli.Exit("❌ Duplicate found. Run `mw notes validate --all` and fix IDs before formatting.", 1)
	}

	hubChanged, noteChanged := 0, 0

	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return fs.SkipDir
			}
			if filepath.Clean(path) == root {
				return nil // never touch root
			}

			changed, err := formatHubNote(path, root)
			if err != nil {
				return err
			}
			if changed {
				hubChanged++
			}
			return nil
		}

		if !formatAllNotes || !shouldProcessNoteFile(path) {
			return nil
		}

		changed, err := formatRegularNote(path, root)
		if err != nil {
			return err
		}
		if changed {
			noteChanged++
		}
		return nil
	}); err != nil {
		return cli.Exit("❌ Format walk failed: "+err.Error(), 1)
	}

	// Rebuild + write registry if clean
	reg2, err := mdregistry.Build(root)
	if err != nil {
		return cli.Exit("❌ Registry rebuild failed: "+err.Error(), 1)
	}

	if len(reg2.MissingHub) == 0 && len(reg2.Duplicates) == 0 {
		if err := mdregistry.WriteToDisk(root, reg2.Registry); err != nil {
			return cli.Exit("❌ Failed to write registry: "+err.Error(), 1)
		}
	}

	logDone(formatAllNotes, hubChanged, noteChanged)
	return nil
}

func logStart(all bool) {
	if all {
		log.Println("🧩 Formatting ALL notes (hub notes + heading/id normalization for .md notes)...")
		return
	}
	log.Println("🧩 Formatting hub files...")
}

func logDone(all bool, hubChanged, noteChanged int) {
	if all {
		log.Printf("✅ Done. Updated %d hub file(s) and %d note file(s).", hubChanged, noteChanged)
		return
	}
	log.Printf("✅ Done. Updated %d hub file(s).", hubChanged)
}

func shouldSkipDir(name string) bool {
	if name == ".git" {
		return true
	}
	return strings.HasPrefix(name, ".")
}

func shouldProcessNoteFile(path string) bool {
	if !strings.EqualFold(filepath.Ext(path), ".md") {
		return false
	}
	if notefiles.IsHubNotePath(path) {
		return false
	}
	return true
}

func formatHubNote(dir, root string) (bool, error) {
	const createMissingChildHubs = true
	changed, issues, err := applyHubUpdate(filepath.Clean(dir), root, createMissingChildHubs)
	for _, msg := range issues {
		log.Printf("⚠️  %s", msg)
	}
	return changed, err
}

func formatRegularNote(path, root string) (bool, error) {
	desiredID := strings.TrimSuffix(filepath.Base(path), ".md")
	idChanged, err := markdown.EnsureMetaIDFile(path, desiredID)
	if err != nil {
		return false, err
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return idChanged, err
	}

	title := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	normalized, headingChanged := normalizeMarkdownHeadings(string(contentBytes), title)
	if !headingChanged {
		return idChanged, nil
	}
	if err := os.WriteFile(path, []byte(normalized), 0o644); err != nil {
		return idChanged, err
	}

	return true, nil
}
