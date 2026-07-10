package mwcli

import (
	"log"
	"os"

	"github.com/Noswad123/mind-weaver/internal/version"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func Run(args []string) error {
	log.SetOutput(os.Stderr)

	env := loadEnv()
	d := deps{
		cfg: config{
			loadErr:            env.loadErr,
			appConfig:          env.appConfig,
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
			buildVersionCommand(),
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
		Version:  version.String(),
		Commands: commands,
	}

	return app.Run(args)
}
