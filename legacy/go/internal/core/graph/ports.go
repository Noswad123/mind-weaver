package graph

import "context"

type Service interface {
	SearchIndex(ctx context.Context, query string, limit, offset int) ([]ConnectednessRow, error)
	ListIndex(ctx context.Context, limit, offset int) ([]ConnectednessRow, error)
	LoadFocus(ctx context.Context, noteID int, neighborsLimit int) (FocusView, error)
	ResolveStartNote(ctx context.Context, query string) (*NoteLite, error)
}
