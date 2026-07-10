package mwcli

import (
	"fmt"
	"strings"

	mwconfig "github.com/Noswad123/mind-weaver/internal/config"
	"github.com/urfave/cli/v2"
)

func buildInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Create a MindWeaver config and local directories",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Usage: "Config file path", Value: mwconfig.DefaultConfigPath()},
			&cli.StringFlag{Name: "notes-dir", Usage: "Root directory for markdown notes"},
			&cli.StringFlag{Name: "db-path", Usage: "SQLite database path"},
			&cli.StringFlag{Name: "inbox", Usage: "Inbox note path"},
			&cli.BoolFlag{Name: "force", Usage: "Overwrite an existing config file"},
		},
		Action: func(c *cli.Context) error {
			cfg, err := mwconfig.Init(mwconfig.InitOptions{
				ConfigPath: c.String("config"),
				NotesDir:   c.String("notes-dir"),
				DBPath:     c.String("db-path"),
				InboxPath:  c.String("inbox"),
				Force:      c.Bool("force"),
			})
			if err != nil {
				return err
			}

			fmt.Fprintf(c.App.Writer, "✅ MindWeaver initialized\n")
			fmt.Fprintf(c.App.Writer, "config: %s\n", cfg.ConfigPath)
			fmt.Fprintf(c.App.Writer, "notes:  %s\n", cfg.NotesDir)
			fmt.Fprintf(c.App.Writer, "db:     %s\n", cfg.DBPath)
			fmt.Fprintf(c.App.Writer, "\nNext steps:\n")
			fmt.Fprintf(c.App.Writer, "  1. Add markdown notes under %s\n", cfg.NotesDir)
			fmt.Fprintf(c.App.Writer, "  2. Run `mw doctor`\n")
			fmt.Fprintf(c.App.Writer, "  3. Run `mw notes ingest` when ready\n")
			return nil
		},
	}
}

func buildConfigCommand() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "Inspect MindWeaver configuration",
		Subcommands: []*cli.Command{
			{
				Name:  "path",
				Usage: "Print the active config path",
				Action: func(c *cli.Context) error {
					cfg, _ := mwconfig.Load()
					fmt.Fprintln(c.App.Writer, cfg.ConfigPath)
					return nil
				},
			},
			{
				Name:  "show",
				Usage: "Print merged configuration",
				Action: func(c *cli.Context) error {
					cfg, err := mwconfig.Load()
					if err != nil {
						return err
					}
					fmt.Fprint(c.App.Writer, mwconfig.Format(cfg))
					return nil
				},
			},
		},
	}
}

func buildDoctorCommand() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Check MindWeaver configuration and local dependencies",
		Action: func(c *cli.Context) error {
			cfg, loadErr := mwconfig.Load()
			checks := mwconfig.Doctor(cfg, loadErr)
			failed := false
			for _, check := range checks {
				if check.Status == mwconfig.CheckFail {
					failed = true
				}
				fmt.Fprintf(c.App.Writer, "%s %-20s %s\n", statusLabel(check.Status), check.Name+":", check.Message)
			}
			if failed {
				return cli.Exit("MindWeaver doctor found blocking issues", 1)
			}
			return nil
		},
	}
}

func statusLabel(status mwconfig.CheckStatus) string {
	switch status {
	case mwconfig.CheckOK:
		return "✅"
	case mwconfig.CheckWarn:
		return "⚠️ "
	case mwconfig.CheckFail:
		return "❌"
	default:
		return strings.ToUpper(string(status))
	}
}
