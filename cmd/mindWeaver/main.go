package main

import (
	"log"
	"fmt"
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
		NoteDBPath:     env.NoteDBPath,
		CheatDBPath:     env.CheatDBPath,
		CheatSchemaPath: env.CheatSchemaPath,
		NoteSchemaPath: env.NoteSchemaPath,
		ConfigPath: env.ConfigPath,
		LoomPath: env.LoomPath,
		PythonPath: env.PythonPath,
	}

	noteDb, err := db.NewNoteDb(env.NoteDBPath, env.NoteSchemaPath)
	cheatDb, err := db.NewCheatDb(env.CheatDBPath, env.CheatSchemaPath)
	if err != nil {
		log.Fatalf("Failed to init db: %v", err)
	}
	defer noteDb.Close()
	defer cheatDb.Close()

	app := &cli.App{
		Name:  "mind-weaver",
		Usage: "Synthesize notes, manage cheatsheets, and more",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "spirits", Aliases: []string{"s"}, Usage: "Interact with spirits from the void (notes) (default)"},
			&cli.BoolFlag{Name: "incatations", Aliases: []string{"i"}, Usage: "Study your incatations from your grimmoire (cheatsheets)"},
		},
		Commands: []*cli.Command{
			{
				Name:  "recite",
				Usage: "Recite your incatations (cheatsheets)",
				Action: func(c *cli.Context) error {
				if env.CheatDBPath == "" || env.CheatSchemaPath == "" {
					return fmt.Errorf("Cheat DB and Schema path required for recite")
				}
					return runner.RunReciteCommand(c, cheatDb)
				},
			},
			{
				Name:  "summon",
				Usage: "Commune with spirits (notes) or Recite your incatations (cheatsheets)",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "id", Usage: "Fetch by ID"},
					&cli.StringFlag{Name: "search", Usage: "Fuzzy search spirits or incantations from your grimmoire"},
					&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
				},
				Action: func(c *cli.Context) error {
					return runner.RunSummonCommand(c, noteDb)
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
					return runner.RunBanishCommand(c, config, noteDb)
				},
			},
			{
				Name:  "loom",
				Usage: "Launch the visual graph tool",
				Action: func(c *cli.Context) error {
					if config.LoomPath == "" || config.PythonPath == "" {
							return fmt.Errorf("❌ Missing required config values: LoomPath or PythonPath")
					}
					return runner.RunLoomCommand(c, config.PythonPath, config.LoomPath)
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
					return runner.RunWatchCommand(c, config, noteDb)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

