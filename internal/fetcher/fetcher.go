
package fetcher

import (
	"fmt"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

type FetchOptions struct {
	SearchInput       string
	Id		 *int
	Tags 	[]string
}


func FetchNotes(opts FetchOptions) ([]parser.ParsedNote, error) {
	var results []parser.ParsedNote
	var err error

	switch {
	case opts.Id != nil:
		note, err := db.GetNoteByID(*opts.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch by ID: %w", err)
		}
		results = append(results, note)

	case opts.SearchInput != "":
		results, err = db.SearchNotesByName(opts.SearchInput)
		if err != nil {
			return nil, fmt.Errorf("failed fuzzy search: %w", err)
		}

	case len(opts.Tags) > 0:
		results, err = db.GetNotesByTags(opts.Tags)
		if err != nil {
			return nil, fmt.Errorf("failed tag search: %w", err)
		}

	default:
		results, err = db.GetAllNotes()
		if err != nil {
			return nil, fmt.Errorf("failed to get all notes: %w", err)
		}
	}

	return results, nil
}