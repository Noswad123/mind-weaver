package ui

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/saved_query"
)

type NoteService interface {
	GetByID(c context.Context, id int)(*note.Note, error)
	SearchByTitle(c context.Context, q string)([]note.Note, error)
	ListByTags(c context.Context, tags []string)([]note.Note, error)
	List(c context.Context, limit int, offset int)([]note.Note, error)
}

type QueryService interface {
	ListSaved(ctx context.Context) ([]savedquery.SavedQuery, error)
	Execute(ctx context.Context, sqlText string) (string, error)
}
