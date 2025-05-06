package main

import (
	"flag"
	"strings"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/writer"
	"github.com/Noswad123/mind-weaver/internal/watcher"
	"github.com/Noswad123/mind-weaver/internal/fetcher"
	"github.com/Noswad123/mind-weaver/internal/formatter"
	_ "github.com/mattn/go-sqlite3"
)

func splitAndTrim(input string) []string {
	raw := strings.Split(input, ",")
	var result []string
	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mw [flags | loom]

Available flags:
`)
		flag.PrintDefaults()
	}

	gaze := flag.Bool("gaze", false, "Run in watch mode (default false)")
	banish := flag.Bool("banish", false, "Resync all notes")
	engrave := flag.Bool("engrave", false, "Ensure all index.norg files exist and are structured correctly")

	summon := flag.Bool("summon", false, "Fetch a note")
	summonId := flag.Int("id", 0, "Fetch note by ID")
	summonSearch := flag.String("search", "", "Fuzzy search notes by name")
	summonTags := flag.String("tags", "", "Comma-separated list of tags to filter notes")

	flag.Parse()

	// handle subcommands like: mw loom
	args := flag.Args()
	if len(args) > 0 && args[0] == "loom" {
    python := os.Getenv("PYTHON_PATH")
    if python == "" {
        python = "python3"
    }
    cmd := exec.Command(python, "scripts/loom/main.py")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("❌ Failed to run visualizer: %v", err)
		}
		os.Exit(0)
	}

	envLoaded := false

	if err := godotenv.Load(".env"); err == nil {
    	envLoaded = true
	} else {
		exePath, _ := os.Executable()
		rootDir := filepath.Dir(filepath.Dir(exePath)) // Go up from ./bin/mw
		log.Println(rootDir)
		if err := godotenv.Load(filepath.Join(rootDir, ".env")); err == nil {
		envLoaded = true
		}
	}

	if !envLoaded {
		log.Fatal("❌ Could not load .env from current or fallback path")
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

	if *banish {
		log.Println("🔁 Sync all notes...")
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
			note := parser.ParseNote(string(content), f)
			db.UpsertNote(note, f)
		}
		log.Printf("✅ synced %d notes\n", len(files))

		if err := writer.WriteWorkspaces(db.Conn, configPath, notesDir); err != nil {
			log.Printf("⚠️ Failed to sync Neorg workspaces: %v\n", err)
		}
	}

	if *engrave {
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
				formatter.FormatIndexNote(dir, notesDir)
			}
		}
	}

if *summon {
	log.Println("🔍 Fetching note(s)")

	var idPtr *int
	if *summonId != 0 {
		idPtr = summonId
	}

	var tags []string
	if *summonTags != "" {
		tags = splitAndTrim(*summonTags)
	}

	opts := fetcher.FetchOptions{
		Id:          idPtr,
		SearchInput: *summonSearch,
		Tags:        tags,
	}

	notes, err := fetcher.FetchNotes(opts)
	if err != nil {
		log.Fatalf("❌ Fetch failed: %v", err)
	}

	if len(notes) == 0 {
		log.Println("⚠️ No notes matched your query.")
		return
	}

	for _, note := range notes {
		fmt.Printf("📄 %s\n", note.Title)
		fmt.Println("Tags:", note.Tags)
		fmt.Println("Links:", note.Links)
		for _, todo := range note.Todos {
			fmt.Printf("  • [%s] %s\n", todo.RawStatus, *todo.Task)
		}
		fmt.Println("---")
	}
}

	if *gaze {
		log.Println("👁 Starting watcher...")
		watcherConfig := watcher.Config{
			NotesDir:  notesDir,
			DBPath:    dbPath,
			SchemaPath: schemaPath,
			ConfigPath: configPath,
		}
		watcher.WatchNotes(watcherConfig)
	}

	if !*summon && !*gaze && !*banish && !*engrave {
		fmt.Println("ℹ️  Nothing to do. Use --gaze, --banish, --summon or --engrave flags.")
		os.Exit(0)
	}
}
