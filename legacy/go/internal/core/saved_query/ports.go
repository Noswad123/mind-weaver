package savedquery

import "context"

type Reader interface {
    ListSaved(ctx context.Context) ([]SavedQuery, error)
}

type Executor interface {
    Execute(ctx context.Context, sql string) (string, error)
}

type Service interface {
    Reader
    Executor
}
