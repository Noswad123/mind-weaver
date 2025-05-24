package runner

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/helper"
	"github.com/Noswad123/mind-weaver/internal/writer"
	"github.com/fsnotify/fsnotify"

)

func RunWatchCommand(c *cli.Context, cfg helper.Config, db *db.NoteDb) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = filepath.Walk(cfg.NotesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("👀 Watching for note changes in:", cfg.NotesDir)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				if strings.HasSuffix(event.Name, ".norg") || strings.HasSuffix(event.Name, ".md") {
					log.Println("📄 Modified:", event.Name)
					content, err := os.ReadFile(event.Name)
					if err != nil {
						log.Printf("❌ Failed to read %s: %v\n", event.Name, err)
						continue
					}
					parsed := parser.ParseNote(string(content), event.Name)
					db.UpsertNote(parsed, event.Name)

					if filepath.Base(event.Name) == "index.norg" {
						log.Println("🔄 index.norg change detected — syncing workspaces...")
						writer.WriteWorkspaces(nil, cfg.ConfigPath, cfg.NotesDir)
					}
				}
			}
			if event.Op&fsnotify.Remove != 0 && filepath.Base(event.Name) == "index.norg" {
				log.Println("🗑 index.norg deleted — syncing workspaces...")
				writer.WriteWorkspaces(nil, cfg.ConfigPath, cfg.NotesDir)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Println("⚠️ Watcher error:", err)
		}
	}
}
