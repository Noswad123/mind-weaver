package main

import (
	"log"
	"os"

	"github.com/Noswad123/mind-weaver/internal/helper"
	"github.com/Noswad123/mind-weaver/internal/runner"
	"github.com/Noswad123/mind-weaver/internal/db"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func main() {
	env := helper.LoadEnv()
	config := helper.Config{
		NotesDir:   env.NotesDir,
		DBPath:     env.DBPath,
		SchemaPath: env.SchemaPath,
		ConfigPath: env.ConfigPath,
	}

	db, err := db.New(env.DBPath, env.SchemaPath)
	if err != nil {
		log.Fatalf("Failed to init db: %v", err)
	}
	defer db.Close()

	app := &cli.App{
		Name:  "mind-weaver",
		Usage: "Synthesize notes, manage cheatsheets, and more",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "spirits", Aliases: []string{"s"}, Usage: "Interact with spirits (notes) (default)"},
			&cli.BoolFlag{Name: "grimmoire", Aliases: []string{"g"}, Usage: "Study your grimmoire and its incantations (cheatsheets)"},
		},
		Commands: []*cli.Command{
			{
				Name:  "summon",
				Usage: "Commune with spirits (notes) or Summon your grimmoire (cheatsheets)",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "interactive", Aliases: []string{"i"}, Usage: "Use TUI for note selection"},
					&cli.IntFlag{Name: "id", Usage: "Fetch by ID"},
					&cli.StringFlag{Name: "search", Usage: "Fuzzy search spirits or incantations from your grimmoire"},
					&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
				},
				Action: func(c *cli.Context) error {
					mode := selectMode(c)
					return runner.RunSummonCommand(c, mode)
				},
			},
			{
				Name:  "engrave",
				Usage: "Ensure cheatsheet index files are structured",
				Flags: []cli.Flag{},
				Action: func(c *cli.Context) error {
					return runner.RunEngraveCommand(c)
				},
			},
			{
				Name:  "banish",
				Usage: "Resync all notes",
				Action: func(c *cli.Context) error {
					return runner.RunBanishCommand(c, config, db)
				},
			},
			{
				Name:  "loom",
				Usage: "Launch the visual graph tool",
				Action: func(c *cli.Context) error {
					return runner.RunLoomCommand(c)
				},
			},
			{
				Name:  "meld",
				Usage: "Compare and merge notes",
				Action: func(c *cli.Context) error {
					return runner.RunMeldCommand(c, config)
				},
			},
			{
				Name:  "watch",
				Usage: "Watch for files changes",
				Action: func(c *cli.Context) error {
					return runner.RunWatchCommand(c, config)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func selectMode(c *cli.Context) string {
	if c.Bool("grimmoire") {
		return "grimmoire"
	}
<<<<<<< HEAD
	return "spirits"
=======
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

func runSummon(opts SummonOptions) ([]parser.ParsedNote, error) {
	if opts.SummonInteractive != nil && *opts.SummonInteractive {
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

func loadEnv() watcher.Config {
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
