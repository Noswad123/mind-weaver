package query

import (
	"context"
	"fmt"
	"strings"

	savedquery "github.com/Noswad123/mind-weaver/internal/core/saved_query"
)

type Store interface {
	LoadSavedQueries(ctx context.Context) ([]savedquery.SavedQuery, error)
	ExecuteSQL(ctx context.Context, sql string) (string, error)
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListSaved(ctx context.Context) ([]savedquery.SavedQuery, error) {
	return s.store.LoadSavedQueries(ctx)
}

func (s *Service) Execute(ctx context.Context, sqlText string) (string, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return "", nil
	}

	// very basic safety: allow only SELECT/WITH/PRAGMA
	up := strings.ToUpper(strings.TrimSpace(sqlText))
	if !(strings.HasPrefix(up, "SELECT") || strings.HasPrefix(up, "WITH") || strings.HasPrefix(up, "PRAGMA")) {
		return "", fmt.Errorf("only read-only queries are allowed (SELECT/WITH/PRAGMA)")
	}

	return s.store.ExecuteSQL(ctx, sqlText)
}
