package notes

import (
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/features/notes/output"
	"github.com/Noswad123/mind-weaver/internal/features/notes/ui"
	"github.com/Noswad123/mind-weaver/internal/core/note"
)

type ViewOptions struct {
	Id           *int
	SearchString *string
	Tags         *string
}


func ViewNotes(c *cli.Context, noteSvc NoteService, querySvc ui.QueryService) error {
	args := c.Args().Slice()
	hasNoArgs := len(args) == 0

	// No args => launch SQL TUI
	if hasNoArgs {
		if err := ui.RunNoteTUI(c.Context, querySvc); err != nil {
			log.Printf("Failed to start TUI: %v", err)
			return cli.Exit("❌ Failed to start TUI", 1)
		}
		os.Exit(0)
	}

	opts := ViewOptions{
		Id:           cliIntPtr(c.Int("id")),
		SearchString: cliStringPtr(c.String("search")),
		Tags:         cliStringPtr(c.String("tags")),
	}

	notes, err := fetchNotes(c, opts, noteSvc)
	if err != nil {
		return cli.Exit(err.Error(), 1)
	}

	// Temporary adapter: output.PrintNotes still expects []db.NoteRow
	output.PrintNotes(notes, output.FormatPretty)
	return nil
}

func fetchNotes(c *cli.Context, opts ViewOptions, noteSvc ui.NoteService) ([]note.Note, error) {
	var idPtr *int
	if opts.Id != nil && *opts.Id != 0 {
		idPtr = opts.Id
	}

	var tags []string
	if opts.Tags != nil && *opts.Tags != "" {
		tags = splitAndTrim(*opts.Tags)
	}

	log.Println("🔍 Fetching note(s)")

	// Priority: id > search > tags > list
	if idPtr != nil {
		n, err := noteSvc.GetByID(c.Context, *idPtr)
		if err != nil {
			return nil, err
		}
		if n == nil {
			return []note.Note{}, nil
		}
		return []note.Note{*n}, nil
	}

	if q := deref(opts.SearchString); q != "" {
		return noteSvc.SearchByTitle(c.Context, q)
	}

	if len(tags) > 0 {
		return noteSvc.ListByTags(c.Context, tags)
	}

	return noteSvc.List(c.Context, 50, 0)
}

func cliStringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func cliIntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func splitAndTrim(input string) []string {
	raw := strings.Split(input, ",")
	var result []string
	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
