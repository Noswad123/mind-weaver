package notes

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/syncops"
	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) UpsertParsedNote(ctx context.Context, note parser.ParsedNote, path string) error {
	return s.store.WithTx(ctx, func(tx *db.Tx) error {
		path = strings.TrimSpace(path)
		if path == "" {
			return fmt.Errorf("path is required")
		}

		noteID, err := tx.UpsertNoteRow(path, note.Title, note.Content, time.Now())
		if err != nil {
			return fmt.Errorf("upsert note row: %w", err)
		}

		if err := tx.ClearNoteChildren(noteID); err != nil {
			return fmt.Errorf("clear note children: %w", err)
		}

		if err := tx.InsertDomains(noteID, note.Domains); err != nil {
			return fmt.Errorf("insert domains: %w", err)
		}

		if err := tx.InsertTags(noteID, note.Tags); err != nil {
			return fmt.Errorf("insert tags: %w", err)
		}

		groupIDMap, err := insertTaskGroups(tx, noteID, note.Todos)
		if err != nil {
			return fmt.Errorf("insert task groups: %w", err)
		}

		if err := insertTodos(tx, noteID, note.Todos, groupIDMap); err != nil {
			return fmt.Errorf("insert todos: %w", err)
		}

		linkRows := toLinkRows(note.Links)
		if err := tx.InsertLinks(noteID, linkRows); err != nil {
			return fmt.Errorf("insert links: %w", err)
		}

		payload, err := marshalNoteSyncPayload(path, note)
		if err != nil {
			return fmt.Errorf("marshal sync payload: %w", err)
		}

		if err := tx.EnqueueOutboxOperation(syncops.EntityNote, path, syncops.OperationUpsert, payload); err != nil {
			return fmt.Errorf("enqueue sync outbox upsert: %w", err)
		}

		return nil
	})
}

type noteSyncPayload struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Domains []string `json:"domains"`
}

func normalizeStringSlice(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func marshalNoteSyncPayload(path string, n parser.ParsedNote) (string, error) {
	p := noteSyncPayload{
		Path:    path,
		Title:   n.Title,
		Content: n.Content,
		Tags:    normalizeStringSlice(n.Tags),
		Domains: normalizeStringSlice(n.Domains),
	}

	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func marshalNoteDeletePayload(path string) (string, error) {
	b, err := json.Marshal(struct {
		Path string `json:"path"`
	}{Path: path})
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func insertTaskGroups(tx *db.Tx, noteID int, todos []parser.Todo) (map[string]int, error) {
	groupIDMap := map[string]int{}

	for _, t := range todos {
		if !t.IsGroup || t.Task == nil {
			continue
		}

		var derivedID *int
		if t.DerivedGroup != nil {
			if id, ok := groupIDMap[*t.DerivedGroup]; ok {
				derivedID = &id
			}
		}

		id, err := tx.InsertTaskGroup(noteID, *t.Task, t.Level, derivedID, t.Status, t.RawStatus, t.Line)
		if err != nil {
			return nil, err
		}

		groupIDMap[*t.Task] = id
	}

	return groupIDMap, nil
}

func insertTodos(tx *db.Tx, noteID int, todos []parser.Todo, groupIDMap map[string]int) error {
	for _, t := range todos {
		if t.IsGroup || t.Task == nil {
			continue
		}

		groupID, ok := groupIDMap[t.Group]
		if !ok {
			continue
		}

		if err := tx.InsertTodo(noteID, groupID, *t.Task, t.Status, t.RawStatus, t.Depth, t.Line); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DeleteNoteByPath(ctx context.Context, path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}

	return s.store.WithTx(ctx, func(tx *db.Tx) error {
		if err := tx.DeleteNoteByPath(path); err != nil {
			return err
		}

		payload, err := marshalNoteDeletePayload(path)
		if err != nil {
			return fmt.Errorf("marshal delete payload: %w", err)
		}

		if err := tx.EnqueueOutboxOperation(syncops.EntityNote, path, syncops.OperationDelete, payload); err != nil {
			return fmt.Errorf("enqueue sync outbox delete: %w", err)
		}

		return nil
	})
}

func (s *Service) List(ctx context.Context, limit, offset int) ([]note.Note, error) {
	rows, err := s.store.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}
	return mapNotes(rows), nil
}

func (s *Service) SearchByTitle(ctx context.Context, q string) ([]note.Note, error) {
	rows, err := s.store.SearchNotesByName(ctx, q)
	if err != nil {
		return nil, err
	}
	return mapNotes(rows), nil
}

func (s *Service) ListByTags(ctx context.Context, tags []string) ([]note.Note, error) {
	rows, err := s.store.ListByTags(ctx, tags)
	if err != nil {
		return nil, err
	}
	return mapNotes(rows), nil
}

func (s *Service) ListByDomain(ctx context.Context, domain string) ([]note.Note, error) {
	rows, err := s.store.ListByDomain(ctx, domain)
	if err != nil {
		return nil, err
	}
	return mapNotes(rows), nil
}

func (s *Service) GetByID(ctx context.Context, id int) (*note.Note, error) {
	r, err := s.store.GetNoteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	n := mapNote(r)
	return &n, nil
}

func (s *Service) GetByUID(ctx context.Context, uid string) (*note.Note, error) {
	noteID, err := s.store.GetNoteIdByUid(ctx, uid)
	if err != nil {
		return nil, err
	}
	if noteID == nil {
		return nil, fmt.Errorf("No note registered with uid: %s", uid)
	}
	r, err := s.store.GetNoteByID(ctx, *noteID)
	if err != nil {
		return nil, err
	}
	n := mapNote(r)
	return &n, nil
}

func (s *Service) GetAllNotePaths(ctx context.Context) (map[string]struct{}, error) {
	paths, err := s.store.GetAllNotePaths(ctx)
	if err != nil {
		return nil, err
	}

	out := map[string]struct{}{}
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out, nil
}
