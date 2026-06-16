package mwcli

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

var (
	flagsForFormat = []cli.Flag{
		&cli.BoolFlag{Name: "all", Usage: "Format all notes, not just hub notes"},
	}

	flagsForGet = []cli.Flag{
		&cli.IntFlag{Name: "id", Usage: "Fetch by ID"},
		&cli.StringFlag{Name: "search", Usage: "Fuzzy search spirits"},
		&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
	}

	flagsForQueryNotes = []cli.Flag{
		&cli.IntFlag{Name: "id", Usage: "Fetch a single note by DB id"},
		&cli.StringFlag{Name: "uid", Usage: "Fetch a single note by uid"},
		&cli.StringFlag{
			Name:  "format",
			Usage: "Output format: json|text",
			Value: "json",
		},
		&cli.StringFlag{
			Name:  "view",
			Usage: "Text view: pretty|commands",
		},
		&cli.StringFlag{Name: "search", Usage: "Search notes by title (substring match)"},
		&cli.StringFlag{Name: "tags", Usage: "Comma-separated tags"},
		&cli.StringFlag{Name: "domain", Usage: "Filter notes by metadata domain (e.g. glossary)"},
		&cli.StringFlag{Name: "category", Usage: "Filter glossary notes by category folder name"},
		&cli.IntFlag{Name: "limit", Value: 50, Usage: "Max results when listing"},
		&cli.IntFlag{Name: "offset", Value: 0, Usage: "Offset when listing"},
	}

	flagsForValidate = []cli.Flag{
		&cli.BoolFlag{Name: "all", Usage: "Validate all notes (currently default behavior)"},
		&cli.StringFlag{Name: "domain", Usage: "Validate notes in a certain domain"},
	}

	flagsForProjectionScope = []cli.Flag{
		&cli.StringSliceFlag{
			Name:    "scope",
			Aliases: []string{"scopes"},
			Usage:   "Scope projection to notes containing all listed domains; may be repeated or comma-separated",
		},
	}

	flagsForQueryGraph = []cli.Flag{
		&cli.StringFlag{Name: "search", Usage: "Seed graph from matching note title/path/tag/domain text"},
		&cli.StringFlag{Name: "domain", Usage: "Seed graph from notes in a domain"},
		&cli.IntFlag{Name: "depth", Value: 1, Usage: "Neighbor expansion depth from matched seed nodes"},
		&cli.IntFlag{Name: "limit", Value: 250, Usage: "Maximum nodes returned"},
	}

	flagsForFix = []cli.Flag{
		&cli.BoolFlag{Name: "json", Usage: "Print conflicts as JSON"},
		&cli.BoolFlag{Name: "cached", Usage: "Use cached conflicts without rescanning"},
		&cli.BoolFlag{Name: "all", Usage: "Include WARN conflicts (not just blocking errors)"},
		&cli.BoolFlag{Name: "no-open", Usage: "Do not open nvim; print selected conflicts"},
		&cli.BoolFlag{Name: "no-fuzzy", Usage: "Skip fuzzy picker and include all conflicts"},
	}

	flagsForInteractive = []cli.Flag{
		&cli.BoolFlag{
			Name:    "interactive",
			Usage:   "Open interactive todo dashboard",
			Aliases: []string{"i"},
		},
	}
)
