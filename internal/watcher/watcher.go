package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/updater"
)

func WatchNotes(notesDir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
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

	dbPath := os.Getenv("DB_PATH")
	schemaPath := os.Getenv("SCHEMA_PATH")
	configPath := os.Getenv("NEORG_CONFIG")

	db.InitDBWithPaths(dbPath, schemaPath)

	log.Println("👀 Watching for note changes in:", notesDir)

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
					parsed := parser.ParseNorg(string(content), event.Name)
					db.UpsertNote(parsed, event.Name)

					if filepath.Base(event.Name) == "index.norg" {
						log.Println("🔄 index.norg change detected — syncing workspaces...")
						updater.SyncNeorgWorkspaces(nil, configPath, notesDir)
					}
				}
			}
			if event.Op&fsnotify.Remove != 0 && filepath.Base(event.Name) == "index.norg" {
				log.Println("🗑 index.norg deleted — syncing workspaces...")
				updater.SyncNeorgWorkspaces(nil, configPath, notesDir)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("⚠️ Watcher error:", err)
		}
	}
}
