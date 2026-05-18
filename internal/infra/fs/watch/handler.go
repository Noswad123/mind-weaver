package watch

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/fsnotify/fsnotify"
	"github.com/urfave/cli/v2"
)

type WatchService interface {
	UpsertParsedNote(ctx context.Context, note parser.ParsedNote, relPath string) error
	DeleteNoteByPath(ctx context.Context, relPath string) error
}

func Run(c *cli.Context, notesDir string, noteSvc WatchService) error {
	root := filepath.Clean(notesDir)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return cli.Exit("❌ Failed to create watcher: "+err.Error(), 1)
	}
	defer watcher.Close()

	// Watch directories (skip .git + hidden)
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if name == ".git" || strings.HasPrefix(name, ".") {
			return fs.SkipDir
		}
		return watcher.Add(path)
	}); err != nil {
		return cli.Exit("❌ Failed to register watch directories: "+err.Error(), 1)
	}

	log.Println("👀 Watching for note changes in:", root)

	debounce := newDebouncer(250 * time.Millisecond)

	for {
		select {
		case <-c.Context.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Remove: delete from DB.
			if event.Op&fsnotify.Remove != 0 {
				handleRemove(c.Context, root, event.Name, noteSvc)
				continue
			}

			// Rename: treat as "probably removed old file". Many editors rename temp files.
			// We can't reliably know the new name from this event, so we just delete old path if it was tracked.
			if event.Op&fsnotify.Rename != 0 {
				handleRename(c.Context, root, event.Name, noteSvc)
				continue
			}

			// Write/Create: (debounced) ingest the file.
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if !isMarkdown(event.Name) {
					continue
				}

				debounce.Do(event.Name, func() {
					if err := ingestOneFile(c.Context, root, event.Name, noteSvc); err != nil {
						log.Printf("❌ %v\n", err)
						return
					}

				})
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Println("⚠️ Watcher error:", err)
		}
	}
}

func handleRemove(ctx context.Context, root, absPath string, noteSvc WatchService) {
	if !isMarkdown(absPath) {
		return
	}

	if rel, ok := relPathFromAbs(root, absPath); ok {
		if err := noteSvc.DeleteNoteByPath(ctx, rel); err != nil {
			log.Printf("⚠️ delete %s: %v\n", rel, err)
		} else {
			log.Println("🗑 Deleted:", rel)
		}
	}
}

func handleRename(ctx context.Context, root, absPath string, noteSvc WatchService) {
	if !isMarkdown(absPath) {
		return
	}

	if rel, ok := relPathFromAbs(root, absPath); ok {
		_ = noteSvc.DeleteNoteByPath(ctx, rel) // best-effort
	}
}

func ingestOneFile(ctx context.Context, root, absPath string, noteSvc WatchService) error {
	rel, ok := relPathFromAbs(root, absPath)
	if !ok {
		return nil
	}

	b, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", rel, err)
	}

	parsed := parser.ParseNote(string(b), rel)
	if err := noteSvc.UpsertParsedNote(ctx, parsed, rel); err != nil {
		return fmt.Errorf("upsert %s: %w", rel, err)
	}

	log.Println("📄 Ingested:", rel)
	return nil
}

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}

func relPathFromAbs(root, absPath string) (string, bool) {
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if strings.HasPrefix(rel, "..") {
		return "", false
	}
	if strings.HasPrefix(rel, ".git/") {
		return "", false
	}
	return rel, true
}
