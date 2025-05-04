package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/indexer"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/updater"
	"github.com/Noswad123/mind-weaver/internal/watcher"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mw [flags]

Available flags:
`)
		flag.PrintDefaults()
	}

	watch := flag.Bool("watch", false, "Run in watch mode (default false)")
	reindex := flag.Bool("reindex", false, "Reindex all notes from scratch")
	ensureIndices := flag.Bool("ensure-indices", false, "Ensure all index.norg files exist and are structured correctly")
	flag.Parse()

	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	notesDir := os.Getenv("NOTES_DIR")
	if notesDir == "" {
		log.Fatal("NOTES_DIR not set in .env file")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		log.Fatal("DB_PATH not set in .env file")
	}

	schemaPath := os.Getenv("SCHEMA_PATH")
	if schemaPath == "" {
		log.Fatal("SCHEMA_PATH not set in .env file")
	}

	configPath := os.Getenv("NEORG_CONFIG")
	if configPath == "" {
		log.Fatal("NEORG_CONFIG not set in .env file")
	}

	db.InitDBWithPaths(dbPath, schemaPath)
	sqlite, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("❌ Unable to reopen DB connection:", err)
	}

	if *reindex {
		log.Println("🔁 Reindexing all notes...")
		files := []string{}
		err := filepath.Walk(notesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (filepath.Ext(path) == ".norg" || filepath.Ext(path) == ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error walking notes directory: %v", err)
		}

		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				log.Printf("❌ Could not read %s: %v", f, err)
				continue
			}
			note := parser.ParseNorg(string(content), f)
			db.UpsertNote(note, f)
		}
		log.Printf("✅ Reindexed %d notes\n", len(files))

		if err := updater.SyncNeorgWorkspaces(sqlite, configPath, notesDir); err != nil {
			log.Printf("⚠️ Failed to sync Neorg workspaces: %v\n", err)
		}
	}
	if *ensureIndices {
		log.Println("🧩 Ensuring index.norg files exist and are structured...")
		entries, err := os.ReadDir(notesDir)
		if err != nil {
			log.Fatalf("❌ Failed to list notes directory: %v", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				dir := filepath.Join(notesDir, entry.Name())
				indexPath := filepath.Join(dir, "index.norg")
				if _, err := os.Stat(indexPath); os.IsNotExist(err) {
					log.Printf("➕ Creating missing index.norg in %s", dir)
					os.WriteFile(indexPath, []byte(""), 0644)
				}
				indexer.EnsureIndex(dir, notesDir)
			}
		}
	}

	if *watch {
		log.Println("👁 Starting watcher...")
		watcher.WatchNotes(notesDir)
	}

	if !*watch && !*reindex && !*ensureIndices {
		fmt.Println("ℹ️  Nothing to do. Use --watch, --reindex, or --ensure-indices flags.")
		os.Exit(0)
	}
}
