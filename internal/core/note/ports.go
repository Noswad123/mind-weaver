package note

import (
	"context"
	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

type Getter interface {
	GetByID(ctx context.Context, id shared.ID) (*Note, error)
	GetLiteByID(ctx context.Context, id shared.ID) (*Lite, error)
}

type Searcher interface {
	SearchByTitle(ctx context.Context, q string, limit, offset int) ([]Lite, error)
}

type LinkLister interface {
	Backlinks(ctx context.Context, id shared.ID, limit, offset int) ([]Lite, error)
	Outlinks(ctx context.Context, id shared.ID, limit, offset int) ([]Lite, error)
}

type ContentGetter interface {
	ContentByID(ctx context.Context, id shared.ID) (string, error)
}

type DocumentLister interface {
	ListDocuments(ctx context.Context) ([]Document, error)
}
