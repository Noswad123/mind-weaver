package mwcli

import (
	"github.com/Noswad123/mind-weaver/internal/features/notes"
	"github.com/Noswad123/mind-weaver/internal/features/registration"
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
				Name:  "registry",
				Usage: "Query registry",
				Action: d.actionWithServices(func(c *cli.Context, d deps, svcs *services) error {
					return registration.QueryRegistry(c, svcs.registry)
				}),
			},
		},
	}
}
