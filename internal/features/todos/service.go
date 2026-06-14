package todos

import (
	"context"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Store interface {
	ListTaskGroupsForNote(noteID int) ([]db.TaskGroupRow, error)
	ListTodosForNote(noteID int) ([]db.TodoRow, error)
	ListTodoProjection(ctx context.Context) ([]db.TodoProjectionRow, error)
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func (s *Service) GetTodosForNote(ctx context.Context, noteID int) ([]parser.Todo, error) {
	groups, err := s.store.ListTaskGroupsForNote(noteID)
	if err != nil {
		return nil, err
	}
	todos, err := s.store.ListTodosForNote(noteID)
	if err != nil {
		return nil, err
	}

	// Map group id -> name
	groupNameByID := map[int]string{}
	for _, g := range groups {
		groupNameByID[g.ID] = g.Name
	}

	out := []parser.Todo{}

	// group entries
	for _, g := range groups {
		name := g.Name
		t := parser.Todo{
			IsGroup:   true,
			Task:      &name,
			Level:     g.Level,
			Status:    g.Status,
			RawStatus: g.RawStatus,
			Line:      g.LineNumber,
		}
		if g.DerivedGroupID != nil {
			if derivedName, ok := groupNameByID[*g.DerivedGroupID]; ok {
				t.DerivedGroup = &derivedName
			}
		}
		out = append(out, t)
	}

	// todo entries
	for _, td := range todos {
		task := td.Task
		groupName := groupNameByID[td.TaskGroupID]
		out = append(out, parser.Todo{
			IsGroup:   false,
			Task:      &task,
			Group:     groupName,
			Status:    td.Status,
			RawStatus: td.RawStatus,
			Depth:     td.Depth,
			Line:      td.LineNumber,
		})
	}

	return out, nil
}

func (s *Service) ListTodoProjection(ctx context.Context) ([]db.TodoProjectionRow, error) {
	return s.store.ListTodoProjection(ctx)
}
