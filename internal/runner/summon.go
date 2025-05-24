package runner

import (
	"os"
	"log"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/helper"
	"github.com/Noswad123/mind-weaver/internal/interactive"
	"github.com/Noswad123/mind-weaver/internal/output"
	"github.com/Noswad123/mind-weaver/internal/fetcher"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

type SummonSpiritOptions struct {
	SummonId          *int
	SummonSearch      *string
	SummonTags        *string
	SummonInteractive *bool
}


type SummonGrimmoireOptions struct {
	Engrave          *bool
}

func RunSummonCommand(c *cli.Context, mode string, db *db.DB) error {
	if mode != "spirits" && mode != "grimmoire" {
		return cli.Exit("Invalid mode. Use 'spirits' or 'grimmoire'.", 1)
	}
	if mode == "grimmoire" {
		log.Println("⚠️ Grimmoire mode is not yet implemented.")
		return cli.Exit("Grimmoire mode is not yet implemented.", 1)
	}

	if mode == "spirits" {
		opts := SummonSpiritOptions{
			SummonId:          helper.CliIntPtr(c.Int("id")),
			SummonSearch:      helper.CliStringPtr(c.String("search")),
			SummonTags:        helper.CliStringPtr(c.String("tags")),
			SummonInteractive: helper.CliBoolPtr(c.Bool("interactive")),
		}

		notes, err := runSummon(opts, db)
		if err != nil {
			return cli.Exit(err.Error(), 1)
		}

		output.PrintNotes(notes, output.FormatPretty)
	}
	return nil
}

func runSummon(opts SummonSpiritOptions, db *db.DB) ([]parser.ParsedNote, error) {
	if opts.SummonInteractive != nil && *opts.SummonInteractive {
		err := interactive.RunTUI(db)
		if err != nil {
			log.Fatalf("Failed to start TUI: %v", err)
		}
		os.Exit(0)
	}
	var idPtr *int
	if opts.SummonId != nil && *opts.SummonId != 0 {
		idPtr = opts.SummonId
	}

	var tags []string
	if opts.SummonTags != nil && *opts.SummonTags != "" {
		tags = helper.SplitAndTrim(*opts.SummonTags)
	}

	fetchOpts := fetcher.FetchOptions{
		Id:          idPtr,
		SearchInput: helper.Deref(opts.SummonSearch),
		Tags:        tags,
	}

	log.Println("🔍 Fetching note(s)")
	notes, err := fetcher.FetchNotes(fetchOpts, db)
	if err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	if len(notes) == 0 {
		log.Println("⚠️ No notes matched your query.")
	}

	return notes, nil
}

