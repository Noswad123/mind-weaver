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
	"github.com/Noswad123/mind-weaver/internal/interactive"
	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/output"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/writer"
	"github.com/Noswad123/mind-weaver/internal/watcher"
	"github.com/Noswad123/mind-weaver/internal/fetcher"
	"github.com/Noswad123/mind-weaver/internal/formatter"
	_ "github.com/mattn/go-sqlite3"
)

type SummonOptions struct {
	SummonId         *int
	SummonSearch     *string
	SummonTags       *string
	SummonInteractive *bool
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

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
	summonInteractive := flag.Bool("interactive", false, "Run Command in interactive mode")
	summonId := flag.Int("id", 0, "Fetch note by ID")
	summonSearch := flag.String("search", "", "Fuzzy search notes by name")
	summonTags := flag.String("tags", "", "Comma-separated list of tags to filter notes")

	flag.Parse()

	// handle subcommands like: mw loom
	args := flag.Args()
	runLoom(args)

	env := loadEnv()

	db.InitDBWithPaths(env.DBPath, env.SchemaPath)

	if *banish {
		runBanish(env)
	}

	if *engrave {
		runEngrave(env)
	}

if *summon {
	summonOpts := SummonOptions{SummonId: summonId, SummonSearch: summonSearch, SummonTags: summonTags, SummonInteractive: summonInteractive}
	notes, err := runSummon(summonOpts)
	if err != nil {
		log.Fatal(err)
	}
	output.PrintNotes(notes, output.FormatPretty) // Or FormatMarkdown/FormatJSON
}

	if *gaze {
		log.Println("👁 Starting watcher...")
		watcher.WatchNotes(env)
	}

	if !*summon && !*gaze && !*banish && !*engrave {
		fmt.Println("ℹ️  Nothing to do. Use --gaze, --banish, --summon or --engrave flags.")
		os.Exit(0)
	}
}

func runLoom(args []string) {
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
}

func runSummon(opts SummonOptions)([]parser.ParsedNote, error) {
	if opts.SummonInteractive !=nil && *opts.SummonInteractive {
		err := interactive.RunTUI(db.Conn)
		if err != nil {
			log.Fatalf("Failed to start TUI: %v", err)
		}
		os.Exit(0)
	}
	var idPtr *int
	if opts.SummonId != nil && *opts.SummonId != 0 {
		idPtr = opts.SummonId
	}

	var tags []string
	if opts.SummonTags != nil && *opts.SummonTags != "" {
		tags = splitAndTrim(*opts.SummonTags)
	}

	fetchOpts := fetcher.FetchOptions{
		Id:          idPtr,
		SearchInput: deref(opts.SummonSearch),
		Tags:        tags,
	}

	log.Println("🔍 Fetching note(s)")
	notes, err := fetcher.FetchNotes(fetchOpts)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	if len(notes) == 0 {
		log.Println("⚠️ No notes matched your query.")
	}

	return notes, nil
}

func loadEnv()watcher.Config {
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
	return watcher.Config {
		NotesDir: notesDir,
		DBPath: dbPath,
		SchemaPath: schemaPath,
		ConfigPath: configPath,
	}
}

func runBanish(env watcher.Config) {
		log.Println("🔁 Sync all notes...")
		files := []string{}
		err := filepath.Walk(env.NotesDir, func(path string, info os.FileInfo, err error) error {
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

		if err := writer.WriteWorkspaces(db.Conn, env.ConfigPath, env.NotesDir); err != nil {
			log.Printf("⚠️ Failed to sync Neorg workspaces: %v\n", err)
		}
}
func runEngrave(env watcher.Config) {
		log.Println("🧩 Ensuring index.norg files exist and are structured...")
		entries, err := os.ReadDir(env.NotesDir)
		if err != nil {
			log.Fatalf("❌ Failed to list notes directory: %v", err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				dir := filepath.Join(env.NotesDir, entry.Name())
				indexPath := filepath.Join(dir, "index.norg")
				if _, err := os.Stat(indexPath); os.IsNotExist(err) {
					log.Printf("➕ Creating missing index.norg in %s", dir)
					os.WriteFile(indexPath, []byte(""), 0644)
				}
				formatter.FormatIndexNote(dir, env.NotesDir)
			}
		}
}
