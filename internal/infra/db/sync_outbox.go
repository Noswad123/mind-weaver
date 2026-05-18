package db

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func operationPayloadHash(entityType syncops.EntityType, entityKey string, opType syncops.OperationType, payload string, baseVersion int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s", entityType, entityKey, opType, baseVersion, payload)))
	return hex.EncodeToString(sum[:])
}

func randomOperationID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (t *Tx) EnqueueOutboxOperation(entityType syncops.EntityType, entityKey string, opType syncops.OperationType, payload string) error {
	baseVersion, err := t.ResolveSyncEntityVersion(entityType, entityKey)
	if err != nil {
		return err
	}

	return t.EnqueueOutboxOperationWithBaseVersion(entityType, entityKey, opType, payload, baseVersion)
}

func (t *Tx) EnqueueOutboxOperationWithBaseVersion(entityType syncops.EntityType, entityKey string, opType syncops.OperationType, payload string, baseVersion int) error {
	entityKey = strings.TrimSpace(entityKey)
	if entityKey == "" {
		return errors.New("entity key is required")
	}
	if baseVersion < 0 {
		return errors.New("base version must be >= 0")
	}

	payloadHash := operationPayloadHash(entityType, entityKey, opType, payload, baseVersion)

	var existingID int64
	err := t.tx.QueryRow(`
		SELECT id
		FROM sync_outbox
		WHERE entity_type = ?
		  AND entity_key = ?
		  AND op_type = ?
		  AND base_version = ?
		  AND payload_hash = ?
		ORDER BY id DESC
		LIMIT 1
	`, entityType, entityKey, opType, baseVersion, payloadHash).Scan(&existingID)
	if err == nil {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	opID, err := randomOperationID()
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = t.tx.Exec(`
		INSERT INTO sync_outbox (
			op_id,
			idempotency_key,
			entity_type,
			entity_key,
			op_type,
			payload,
			payload_hash,
			base_version,
			status,
			attempt_count,
			created_at,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, opID, opID, entityType, entityKey, opType, payload, payloadHash, baseVersion, syncops.StatusPending, now, now)

	return err
}

func (db *NoteDb) ListPendingOutbox(ctx context.Context, limit int) ([]SyncOutboxRow, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT
			id,
			op_id,
			idempotency_key,
			entity_type,
			entity_key,
			op_type,
			COALESCE(payload, ''),
			payload_hash,
			base_version,
			status,
			attempt_count,
			COALESCE(last_error, ''),
			COALESCE(created_at, ''),
			COALESCE(updated_at, ''),
			acked_at
		FROM sync_outbox
		WHERE status = ?
		ORDER BY id ASC
		LIMIT ?
	`, syncops.StatusPending, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SyncOutboxRow, 0, limit)
	for rows.Next() {
		var r SyncOutboxRow
		if err := rows.Scan(
			&r.ID,
			&r.OpID,
			&r.IdempotencyKey,
			&r.EntityType,
			&r.EntityKey,
			&r.OpType,
			&r.Payload,
			&r.PayloadHash,
			&r.BaseVersion,
			&r.Status,
			&r.AttemptCount,
			&r.LastError,
			&r.CreatedAt,
			&r.UpdatedAt,
			&r.AckedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}

	return out, rows.Err()
}

func (db *NoteDb) MarkOutboxAcked(ctx context.Context, opID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.ExecContext(ctx, `
		UPDATE sync_outbox
		SET status = ?,
			acked_at = ?,
			updated_at = ?,
			last_error = NULL
		WHERE op_id = ?
	`, syncops.StatusAcked, now, now, opID)
	return err
}

func (db *NoteDb) MarkOutboxAttemptFailed(ctx context.Context, opID, failureReason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.ExecContext(ctx, `
		UPDATE sync_outbox
		SET status = ?,
			attempt_count = attempt_count + 1,
			last_error = ?,
			updated_at = ?
		WHERE op_id = ?
	`, syncops.StatusPending, failureReason, now, opID)
	return err
}

func (db *NoteDb) SetSyncState(ctx context.Context, key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO sync_state (key, value, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			updated_at = excluded.updated_at
	`, key, value, now)
	return err
}

func (db *NoteDb) GetSyncState(ctx context.Context, key string) (*string, error) {
	var value string
	err := db.conn.QueryRowContext(ctx, `SELECT value FROM sync_state WHERE key = ?`, key).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &value, nil
}
