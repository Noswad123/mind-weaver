package validation

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/shared"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Service struct {
	db *db.NoteDb
}

func New(noteDb *db.NoteDb) *Service {
	return &Service{db: noteDb}
}

func (s *Service) ListDocuments(ctx context.Context) ([]note.Document, error) {
	rows, err := s.db.ListNotesForDomainValidation()
	if err != nil {
		return nil, err
	}

	out := make([]note.Document, 0, len(rows))
	for _, r := range rows {
		out = append(out, note.Document{
			ID:      shared.ID(r.ID),
			Path:    r.Path,
			Content: r.Content,
		})
	}
	return out, nil
}
