package mwcli

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func shortcutsFromSubcommands(parentName string, subCommands []*cli.Command) []*cli.Command {
	var out []*cli.Command

	for _, subCommand := range subCommands {
		for _, a := range subCommand.Aliases {
			clone := *subCommand
			clone.Name = a
			clone.Usage = fmt.Sprintf("%s (shortcut for `%s %s`)", subCommand.Usage, parentName, subCommand.Name)
			clone.Aliases = nil
			out = append(out, &clone)
		}
	}

	return out
}
