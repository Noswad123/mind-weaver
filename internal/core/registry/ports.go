package registry

import (
	"context"
	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

type Reader interface {
	ListEntries(ctx context.Context) ([]Entry, error)
	ListConflicts(ctx context.Context) ([]Conflict, error)
}

type Updater interface {
	NoteIDByPath(ctx context.Context, path string) (*shared.ID, error)
	ReplaceRegistry(ctx context.Context, entries []Entry, conflicts []Conflict) error
}
