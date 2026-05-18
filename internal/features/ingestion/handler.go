package ingestion

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/urfave/cli/v2"
)

type NoteManager interface {
	UpsertParsedNote(ctx context.Context, note parser.ParsedNote, path string) error
	GetAllNotePaths(ctx context.Context) (map[string]struct{}, error)
	DeleteNoteByPath(ctx context.Context, path string) error
}

func Run(c *cli.Context, notesDir string, mgr NoteManager) error {
	log.Println("🔁 Ingesting all notes into DB...")

	root := filepath.Clean(notesDir)
	count := 0
	diskPaths := map[string]struct{}{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		diskPaths[rel] = struct{}{}

		if strings.HasPrefix(rel, ".git/") {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}

		note := parser.ParseNote(string(b), rel)
		if err := mgr.UpsertParsedNote(c.Context, note, rel); err != nil {
			return fmt.Errorf("upsert %s: %w", rel, err)
		}

		count++
		return nil
	})
	if err != nil {
		return cli.Exit("❌ Ingest failed: "+err.Error(), 1)
	}

	dbPaths, pruneErr := mgr.GetAllNotePaths(c.Context)
	if pruneErr != nil {
		return cli.Exit("❌ Failed to get note paths from DB: "+pruneErr.Error(), 1)
	}

	prunedCount := 0
	for path := range dbPaths {
		if _, onDisk := diskPaths[path]; !onDisk {
			if deleteNoteErr := mgr.DeleteNoteByPath(c.Context, path); deleteNoteErr != nil {
				log.Printf("Failed to prune %s: %v", path, deleteNoteErr)
				continue
			}
			prunedCount++
		}
	}

	log.Printf("✅ Pruned %d notes", prunedCount)

	log.Printf("✅ ingested %d notes\n", count)
	return nil
}
