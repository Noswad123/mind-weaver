package mwcli

import (
	"log"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/formatter"
	"github.com/Noswad123/mind-weaver/internal/features/graph"
	"github.com/Noswad123/mind-weaver/internal/features/ingestion"
	"github.com/Noswad123/mind-weaver/internal/features/notes"
	"github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/Noswad123/mind-weaver/internal/features/validation"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/Noswad123/mind-weaver/internal/infra/fs/watch"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func buildNotesCommand(d deps) *cli.Command {
	return &cli.Command{
		Name:  "notes",
		Usage: "Notes workflows (query, sync, watch, format, graph, convert)",
		Subcommands: []*cli.Command{
			{
				Name:    "get",
				Aliases: []string{"summon"},
				Usage:   "Retrieve your notes",
				Flags:   flagsForGet,
				Action: d.actionWithServices(func(c *cli.Context, _ deps, svcs *services) error {
					return notes.ViewNotes(c, svcs.note, svcs.query)
				}),
			},
			{
				Name:    "sync",
				Aliases: []string{"seal"},
				Usage:   "Format, ingest, register, and validate registry conflicts",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return syncNotes(c, d.cfg.notesDir, svcs)
				}),
			},
			{
				Name:    "ingest",
				Aliases: []string{"banish"},
				Usage:   "Syncs all notes with db",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  "prune",
						Usage: "Prune notes from the DB that are no longer on disk",
					},
				},
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return ingestion.Run(c, d.cfg.notesDir, svcs.note)
				}),
			},
			{
				Name:    "format",
				Aliases: []string{"meld"},
				Usage:   "Format notes",
				Flags:   flagsForFormat,
				Action: d.action(func(c *cli.Context, d deps) error {
					return formatter.Run(c, d.cfg.notesDir)
				}),
			},
			{
				Name:    "graph",
				Aliases: []string{"loom"},
				Usage:   "Launch the visual graph tool",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return graph.ViewNotesGraph(c, svcs.graph)
				}),
			},
			{
				Name:  "register",
				Usage: "Register note IDs into the DB (note_ids + note_id_conflicts)",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return registration.RegisterNotes(c, d.cfg.notesDir, svcs.registry)
				}),
			},
			{
				Name:  "watch",
				Usage: "Watch for file changes",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return watch.Run(c, d.cfg.notesDir, svcs.note)
				}),
			},
			{
				Name:  "validate",
				Usage: "Validate note files on disk",
				Flags: flagsForValidate,
				Action: d.action(func(c *cli.Context, d deps) error {
					domain := strings.TrimSpace(c.String("domain"))
					if domain == "" {
						return validation.Run(c, d.cfg.notesDir, nil)
					}

					return withNoteDb(d.cfg, func(noteDb *db.NoteDb) error {
						svcs := d.initServices(noteDb)
						return validation.Run(c, d.cfg.notesDir, svcs.validation)
					})
				}),
			},
			{
				Name:    "validate-registry",
				Aliases: []string{"validate-db"},
				Usage:   "Validate DB-backed registry conflicts",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return validation.RunRegistry(c, d.cfg.notesDir, svcs.registry)
				}),
			},
			{
				Name:  "fix",
				Usage: "Fuzzy-pick conflict files and open in a quickfix-capable editor",
				Flags: flagsForFix,
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return validation.Fix(c, d.cfg.notesDir, svcs.registry)
				}),
			},
		},
	}
}

func syncNotes(c *cli.Context, notesDir string, svcs *services) error {
	log.Println("🧙 Running notes pipeline: format → ingest → register → validate-registry")
	if err := formatter.RunAll(notesDir); err != nil {
		return err
	}

	if err := ingestion.Run(c, notesDir, svcs.note); err != nil {
		return err
	}

	if err := registration.RegisterNotes(c, notesDir, svcs.registry); err != nil {
		return err
	}

	if err := validation.RunRegistry(c, notesDir, svcs.registry); err != nil {
		return err
	}

	log.Println("✅ Notes have been formatted, ingested, registered, and registry-validated")
	return nil
}
