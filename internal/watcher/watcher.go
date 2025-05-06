package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/writer"
	"github.com/fsnotify/fsnotify"
)

type Config struct {
	NotesDir   string
	DBPath     string
	SchemaPath string
	ConfigPath string
}

func WatchNotes(cfg Config) {
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
				return
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
				return
			}
			log.Println("⚠️ Watcher error:", err)
		}
	}
}
