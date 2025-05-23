package runner

import (
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/fetcher"
	"github.com/Noswad123/mind-weaver/internal/helper"
	"github.com/Noswad123/mind-weaver/internal/interactive"
	"github.com/Noswad123/mind-weaver/internal/output"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

type SummonSpiritOptions struct {
	SummonId     *int
	SummonSearch *string
	SummonTags   *string
}

type SummonGrimmoireOptions struct {
	Engrave *bool
}

func RunSummonCommand(c *cli.Context, noteDb *db.NoteDb) error {
	args := c.Args().Slice()
	hasNoArgs := len(args) == 0

	if hasNoArgs {
		if err := interactive.RunNoteTUI(noteDb); err != nil {
			log.Fatalf("Failed to start TUI: %v", err)
		}
		os.Exit(0)
	}

	opts := SummonSpiritOptions{
		SummonId:     helper.CliIntPtr(c.Int("id")),
		SummonSearch: helper.CliStringPtr(c.String("search")),
		SummonTags:   helper.CliStringPtr(c.String("tags")),
	}

	notes, err := summonSpirits(opts, noteDb)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	output.PrintNotes(notes, output.FormatPretty)
	return nil
}

func summonSpirits(opts SummonSpiritOptions, db *db.NoteDb) ([]parser.ParsedNote, error) {

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
