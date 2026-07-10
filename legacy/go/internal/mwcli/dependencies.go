package mwcli

import (
	"fmt"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/registry"
	"github.com/Noswad123/mind-weaver/internal/features/graph"
	"github.com/Noswad123/mind-weaver/internal/features/notes"
	"github.com/Noswad123/mind-weaver/internal/features/query"
	"github.com/Noswad123/mind-weaver/internal/features/recipes"
	"github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/Noswad123/mind-weaver/internal/features/todos"
	"github.com/Noswad123/mind-weaver/internal/features/validation"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

type deps struct {
	cfg config
}

func (d deps) action(fn func(ctx *cli.Context, d deps) error) cli.ActionFunc {
	return func(c *cli.Context) error {
		if d.cfg.loadErr != nil {
			return d.cfg.loadErr
		}
		return fn(c, d)
	}
}

func withNoteDb(cfg config, fn func(noteDb *db.NoteDb) error) error {
	if cfg.notesDBPath == "" {
		return fmt.Errorf("NotesDBPath is required")
	}

	noteDb, err := db.NewNoteDb(cfg.notesDBPath, cfg.notesSchemaPath)
	if err != nil {
		return fmt.Errorf("failed to init notes db: %w", err)
	}
	defer noteDb.Close()

	return fn(noteDb)
}

type services struct {
	note       *notes.Service
	query      *query.Service
	graph      *graph.Service
	registry   *registration.Service
	recipe     *recipes.Service
	todo       *todos.Service
	validation *validation.Service
}

func (d deps) initServices(noteDb *db.NoteDb) *services {
	return &services{
		note:       notes.New(noteDb),
		query:      query.New(noteDb),
		graph:      graph.New(noteDb),
		registry:   registration.New(noteDb),
		recipe:     recipes.New(noteDb),
		todo:       todos.New(noteDb),
		validation: validation.New(noteDb),
	}
}

func (d deps) actionWithServices(fn func(ctx *cli.Context, d deps, svcs *services) error) cli.ActionFunc {
	return func(c *cli.Context) error {
		if d.cfg.loadErr != nil {
			return d.cfg.loadErr
		}
		return withNoteDb(d.cfg, func(noteDb *db.NoteDb) error {

			svcs := d.initServices(noteDb)
			return fn(c, d, svcs)
		})
	}
}

type validationServices struct {
	registry.Reader
	note.DocumentLister
}
