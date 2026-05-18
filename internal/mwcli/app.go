package mwcli

import (
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func Run(args []string) error {
	log.SetOutput(os.Stderr)

	env := loadEnv()
	d := deps{
		cfg: config{
			loadErr:            env.loadErr,
			notesDir:           env.notesDir,
			notesDBPath:        env.notesDBPath,
			commandsDBPath:     env.commandsDBPath,
			commandsSchemaPath: env.commandsSchemaPath,
			notesSchemaPath:    env.notesSchemaPath,
			inboxPath:          env.inboxPath,
			dashboardPath:      env.dashboardPath,
		},
	}

	notesCommand := buildNotesCommand(d)
	notesShorcuts := shortcutsFromSubcommands(notesCommand.Name, notesCommand.Subcommands)

	commands := append(
		[]*cli.Command{
			buildInitCommand(),
			buildDoctorCommand(),
			buildConfigCommand(),
			notesCommand,
			buildSyncCommand(d),
			buildQueryCommand(d),
			buildTodosCommand(d),
		},
		notesShorcuts...,
	)

	app := &cli.App{
		Name:     "mw",
		Usage:    "Synthesize notes, manage cheatsheets, and more",
		Commands: commands,
	}

	return app.Run(args)
}
