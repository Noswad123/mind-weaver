package mwcli

import (
	"github.com/Noswad123/mind-weaver/internal/features/notes"
	"github.com/Noswad123/mind-weaver/internal/features/recipes"
	"github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/Noswad123/mind-weaver/internal/features/todos"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func buildQueryCommand(d deps) *cli.Command {
	return &cli.Command{
		Name:  "query",
		Usage: "Query things",
		Subcommands: []*cli.Command{
			{
				Name:  "notes",
				Usage: "Query notes",
				Flags: flagsForQueryNotes,
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return notes.QueryNotes(c, svcs.note)
				}),
			},
			{
				Name:  "domains",
				Usage: "List all note domains",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return notes.QueryDomains(c, svcs.note)
				}),
			},
			{
				Name:  "todos",
				Usage: "List todos from task-index notes",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return todos.QueryTodos(c, svcs.todo)
				}),
			},
			{
				Name:  "recipes",
				Usage: "List recipe projections",
				Flags: flagsForProjectionScope,
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return recipes.QueryRecipes(c, svcs.recipe)
				}),
			},
			{
				Name:      "projection",
				Aliases:   []string{"projections"},
				Usage:     "Query a projection by structural domain",
				ArgsUsage: "<projection>",
				Flags:     flagsForProjectionScope,
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return recipes.QueryProjection(c, svcs.recipe)
				}),
			},
			{
				Name:  "ingredients",
				Usage: "List ingredients or recipe ingredient mentions",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "mentions", Usage: "List ingredient mentions instead of canonical ingredients"},
					&cli.BoolFlag{Name: "unresolved", Usage: "List unresolved ingredient mentions"},
				},
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return recipes.QueryIngredients(c, svcs.recipe)
				}),
			},
			{
				Name:  "registry",
				Usage: "Query registry",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return registration.QueryRegistry(c, svcs.registry)
				}),
			},
		},
	}
}
