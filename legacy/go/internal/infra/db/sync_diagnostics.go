package db

import (
	"context"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func (db *NoteDb) GetSyncDiagnostics(ctx context.Context, cursorStateKey string) (SyncDiagnostics, error) {
	diag := SyncDiagnostics{LocalCursor: "0"}

	queries := []struct {
		sql  string
		scan func() any
	}{
		{
			sql:  `SELECT COUNT(*) FROM sync_outbox WHERE status = ?`,
			scan: func() any { return &diag.PendingOutboxCount },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_outbox WHERE status = ? AND attempt_count > 0`,
			scan: func() any { return &diag.PendingOutboxRetriedCount },
		},
		{
			sql:  `SELECT COALESCE(MAX(attempt_count), 0) FROM sync_outbox WHERE status = ?`,
			scan: func() any { return &diag.PendingOutboxMaxAttemptCount },
		},
		{
			sql:  `SELECT COALESCE(MIN(created_at), '') FROM sync_outbox WHERE status = ?`,
			scan: func() any { return &diag.PendingOutboxOldestCreatedAt },
		},
		{
			sql: `
				SELECT COALESCE((
					SELECT last_error
					FROM sync_outbox
					WHERE status = ?
					  AND attempt_count > 0
					ORDER BY updated_at DESC, id DESC
					LIMIT 1
				), '')
			`,
			scan: func() any { return &diag.PendingOutboxLatestFailure },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_outbox WHERE status = ?`,
			scan: func() any { return &diag.AckedOutboxCount },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_conflicts`,
			scan: func() any { return &diag.TotalConflictCount },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_conflicts WHERE resolved_at IS NULL`,
			scan: func() any { return &diag.UnresolvedConflictCount },
		},
		{
			sql:  `SELECT COALESCE(MIN(created_at), '') FROM sync_conflicts WHERE resolved_at IS NULL`,
			scan: func() any { return &diag.OldestUnresolvedConflictAt },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_entity_versions`,
			scan: func() any { return &diag.SyncEntityVersionCount },
		},
		{
			sql:  `SELECT COUNT(*) FROM sync_todos`,
			scan: func() any { return &diag.SyncedTodoCount },
		},
	}

	for i, q := range queries {
		var err error
		switch i {
		case 0, 1, 2, 3, 4:
			err = db.conn.QueryRowContext(ctx, q.sql, syncops.StatusPending).Scan(q.scan())
		case 5:
			err = db.conn.QueryRowContext(ctx, q.sql, syncops.StatusAcked).Scan(q.scan())
		default:
			err = db.conn.QueryRowContext(ctx, q.sql).Scan(q.scan())
		}
		if err != nil {
			return SyncDiagnostics{}, err
		}
	}

	stateKey := strings.TrimSpace(cursorStateKey)
	if stateKey == "" {
		stateKey = "last_server_cursor"
	}

	cursor, err := db.GetSyncState(ctx, stateKey)
	if err != nil {
		return SyncDiagnostics{}, err
	}
	if cursor != nil {
		trimmed := strings.TrimSpace(*cursor)
		if trimmed != "" {
			diag.LocalCursor = trimmed
		}
	}

	diag.PendingOutboxLatestFailure = strings.TrimSpace(diag.PendingOutboxLatestFailure)
	diag.PendingOutboxOldestCreatedAt = strings.TrimSpace(diag.PendingOutboxOldestCreatedAt)
	diag.OldestUnresolvedConflictAt = strings.TrimSpace(diag.OldestUnresolvedConflictAt)

	return diag, nil
}
