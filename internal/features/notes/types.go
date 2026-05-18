package notes

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Store interface {
	WithTx(ctx context.Context, fn func(*db.Tx) error) error
	DeleteNoteByPath(ctx context.Context, path string) error
	GetAllNotePaths(ctx context.Context) ([]string, error)

	// query/read methods
	List(ctx context.Context, limit, offset int) ([]db.NoteRow, error)
	SearchNotesByName(ctx context.Context, input string) ([]db.NoteRow, error)
	ListByTags(ctx context.Context, tags []string) ([]db.NoteRow, error)
	ListByDomain(ctx context.Context, domain string) ([]db.NoteRow, error)
	GetNoteByID(ctx context.Context, id int) (db.NoteRow, error)
	GetNoteIdByUid(ctx context.Context, uid string) (*int, error)
}

type Service struct {
	store Store
}
