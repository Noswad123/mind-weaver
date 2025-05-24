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
	return "spirits"
}
