package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func (db *NoteDb) InsertSyncConflict(ctx context.Context, event syncops.ConflictEvent) error {
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO sync_conflicts (
			entity_type,
			entity_key,
			local_payload,
			remote_payload,
			reason,
			created_at
		) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, event.EntityType, event.EntityKey, event.LocalPayload, event.RemotePayload, event.Reason)
	return err
}

func (db *NoteDb) ListSyncConflicts(ctx context.Context, limit int) ([]SyncConflictRow, error) {
	return db.ListSyncConflictsByFilter(ctx, SyncConflictFilter{Limit: limit})
}

func (db *NoteDb) ListSyncConflictsByFilter(ctx context.Context, filter SyncConflictFilter) ([]SyncConflictRow, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	var query strings.Builder
	query.WriteString(`
		SELECT
			id,
			entity_type,
			entity_key,
			COALESCE(local_payload, ''),
			COALESCE(remote_payload, ''),
			reason,
			COALESCE(created_at, ''),
			resolved_at
		FROM sync_conflicts
		WHERE 1=1
	`)

	args := make([]any, 0, 2)
	if filter.UnresolvedOnly {
		query.WriteString(` AND resolved_at IS NULL`)
	}
	if filter.CreatedBefore != nil {
		query.WriteString(` AND datetime(created_at) <= datetime(?)`)
		args = append(args, filter.CreatedBefore.UTC().Format(time.RFC3339))
	}

	query.WriteString(` ORDER BY id DESC LIMIT ?`)
	args = append(args, filter.Limit)

	rows, err := db.conn.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SyncConflictRow, 0, filter.Limit)
	for rows.Next() {
		var r SyncConflictRow
		if err := rows.Scan(
			&r.ID,
			&r.EntityType,
			&r.EntityKey,
			&r.LocalPayload,
			&r.RemotePayload,
			&r.Reason,
			&r.CreatedAt,
			&r.ResolvedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}

	return out, rows.Err()
}

func (db *NoteDb) MarkSyncConflictsResolved(ctx context.Context, ids []int64, resolvedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	resolvedAtValue := resolvedAt.UTC().Format(time.RFC3339)
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("sync conflict id must be > 0")
		}
		if _, err := db.conn.ExecContext(ctx, `
			UPDATE sync_conflicts
			SET resolved_at = ?
			WHERE id = ?
		`, resolvedAtValue, id); err != nil {
			return err
		}
	}

	return nil
}
