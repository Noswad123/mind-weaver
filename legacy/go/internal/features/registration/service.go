package registration

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/core/registry"
	"github.com/Noswad123/mind-weaver/internal/core/shared"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Service struct {
	db *db.NoteDb
}

func New(noteDb *db.NoteDb) *Service {
	return &Service{db: noteDb}
}

func (s *Service) ListEntries(ctx context.Context) ([]registry.Entry, error) {
	rows, err := s.db.GetNoteIDRegistry()
	if err != nil {
		return nil, err
	}

	out := make([]registry.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, registry.Entry{
			NoteID:    shared.ID(r.NoteID),
			UID:       r.UID,
			Path:      r.Path,
			IsHub:     r.IsHub,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) ListConflicts(ctx context.Context) ([]registry.Conflict, error) {
	rows, err := s.db.GetNoteIDConflicts()
	if err != nil {
		return nil, err
	}

	out := make([]registry.Conflict, 0, len(rows))
	for _, r := range rows {
		out = append(out, registry.Conflict{
			NoteID:     toNoteIDPtr(r.NoteID),
			UID:        r.UID, // assuming r.UID is *string from conflicts query
			Path:       r.Path,
			IsHub:      r.IsHub,
			Reason:     r.Reason,
			DetectedAt: r.DetectedAt,
		})
	}
	return out, nil
}

// NoteIDByPath resolves the DB note_id for a given path and returns a core shared.ID pointer.
func (s *Service) NoteIDByPath(ctx context.Context, path string) (*shared.ID, error) {
	id, err := s.db.NoteIDByPath(path)
	if err != nil || id == nil {
		return nil, err
	}
	nid := shared.ID(*id)
	return &nid, nil
}

// ReplaceRegistry maps core entries/conflicts to db structs and writes them.
func (s *Service) ReplaceRegistry(ctx context.Context, entries []registry.Entry, conflicts []registry.Conflict) error {
	dbEntries := make([]db.RegisteredNoteID, 0, len(entries))
	for _, e := range entries {
		id := int(e.NoteID)
		dbEntries = append(dbEntries, db.RegisteredNoteID{
			NoteID: &id,
			UID:    e.UID,
			Path:   e.Path,
			IsHub:  e.IsHub,
		})
	}

	dbConflicts := make([]db.RegistryConflict, 0, len(conflicts))
	for _, c := range conflicts {
		dbConflicts = append(dbConflicts, db.RegistryConflict{
			NoteID: noteIDPtrToIntPtr(c.NoteID),
			UID:    strPtrOrEmpty(c.UID),
			Path:   c.Path,
			IsHub:  c.IsHub,
			Reason: c.Reason,
		})
	}

	return s.db.ReplaceRegistry(dbEntries, dbConflicts)
}
