package mwcli

import (
	"fmt"

	"github.com/Noswad123/mind-weaver/internal/version"
	"github.com/urfave/cli/v2"
)

func buildVersionCommand() *cli.Command {
	return &cli.Command{
		Name:  "version",
		Usage: "Print MindWeaver version information",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "short", Usage: "Print only the semantic version"},
		},
		Action: func(c *cli.Context) error {
			if c.Bool("short") {
				fmt.Fprintln(c.App.Writer, version.String())
				return nil
			}
			fmt.Fprintln(c.App.Writer, version.Detailed())
			return nil
		},
	}
}
