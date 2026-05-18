package syncapi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) EnsureSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS sync_devices (
  device_id TEXT PRIMARY KEY,
  device_name TEXT,
  platform TEXT,
  app_version TEXT,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sync_operations (
  seq BIGSERIAL PRIMARY KEY,
  op_id TEXT NOT NULL UNIQUE,
  idempotency_key TEXT NOT NULL,
  device_id TEXT NOT NULL,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_id TEXT NOT NULL,
  op_type TEXT NOT NULL CHECK(op_type IN ('upsert', 'delete')),
  payload TEXT,
  client_updated_at TEXT,
  base_version BIGINT,
  applied BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(device_id, idempotency_key)
);

ALTER TABLE sync_operations ADD COLUMN IF NOT EXISTS applied BOOLEAN NOT NULL DEFAULT TRUE;

CREATE INDEX IF NOT EXISTS idx_sync_operations_seq ON sync_operations(seq);
CREATE INDEX IF NOT EXISTS idx_sync_operations_entity ON sync_operations(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_sync_operations_applied_seq ON sync_operations(applied, seq);

CREATE TABLE IF NOT EXISTS sync_entities (
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_id TEXT NOT NULL,
  version BIGINT NOT NULL,
  last_seq BIGINT NOT NULL,
  last_op_id TEXT NOT NULL,
  last_device_id TEXT NOT NULL,
  last_client_updated_at TEXT,
  op_type TEXT NOT NULL CHECK(op_type IN ('upsert', 'delete')),
  payload TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY(entity_type, entity_id)
);

CREATE TABLE IF NOT EXISTS sync_conflicts (
  id BIGSERIAL PRIMARY KEY,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  winner_op_id TEXT NOT NULL,
  loser_op_id TEXT NOT NULL,
  winner_device_id TEXT NOT NULL,
  loser_device_id TEXT NOT NULL,
  winner_client_updated_at TEXT,
  loser_client_updated_at TEXT,
  winner_seq BIGINT,
  loser_seq BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sync_conflicts_entity ON sync_conflicts(entity_type, entity_id, created_at DESC);
`

	_, err := s.db.ExecContext(ctx, ddl)
	return err
}

func (s *PostgresStore) RegisterDevice(ctx context.Context, req deviceRegistration) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_devices (device_id, device_name, platform, app_version, last_seen_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT(device_id)
		DO UPDATE SET
			device_name = EXCLUDED.device_name,
			platform = EXCLUDED.platform,
			app_version = EXCLUDED.app_version,
			last_seen_at = NOW()
	`, req.DeviceID, req.DeviceName, req.Platform, req.AppVersion)
	return err
}

func (s *PostgresStore) Push(ctx context.Context, deviceID string, operations []syncOperation) (pushResult, error) {
	accepted := make([]string, 0, len(operations))
	conflicts := make([]conflictEvent, 0)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return pushResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, op := range operations {
		accepted = append(accepted, op.OpID)

		var existingSeq int64
		err := tx.QueryRowContext(ctx, `
			SELECT seq
			FROM sync_operations
			WHERE device_id = $1 AND idempotency_key = $2
			LIMIT 1
		`, deviceID, op.IdempotencyKey).Scan(&existingSeq)
		if err == nil {
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return pushResult{}, fmt.Errorf("check idempotency %s: %w", op.OpID, err)
		}

		current, hasCurrent, err := s.loadEntityStateForUpdate(ctx, tx, op.EntityType, op.EntityID)
		if err != nil {
			return pushResult{}, fmt.Errorf("load current entity state for %s: %w", op.OpID, err)
		}

		isConflict := hasCurrent && op.BaseVersion != current.Version
		incomingWins := true
		if isConflict {
			incomingWins = incomingWinsByLWW(op, deviceID, current)
		}
		applied := !isConflict || incomingWins

		var seq int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO sync_operations (
				op_id,
				idempotency_key,
				device_id,
				entity_type,
				entity_id,
				op_type,
				payload,
				client_updated_at,
				base_version,
				applied,
				created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
			RETURNING seq
		`,
			op.OpID,
			op.IdempotencyKey,
			deviceID,
			op.EntityType,
			op.EntityID,
			op.OpType,
			string(op.Payload),
			op.ClientUpdated,
			op.BaseVersion,
			applied,
		).Scan(&seq)
		if err != nil {
			return pushResult{}, fmt.Errorf("insert sync operation %s: %w", op.OpID, err)
		}

		if applied {
			nextVersion := 1
			if hasCurrent {
				nextVersion = current.Version + 1
			}

			if err := s.upsertEntityState(ctx, tx, op, deviceID, seq, nextVersion); err != nil {
				return pushResult{}, fmt.Errorf("upsert entity state %s: %w", op.OpID, err)
			}
		}

		if isConflict {
			event := conflictEvent{
				EntityType: op.EntityType,
				EntityID:   op.EntityID,
				Reason:     "base_version_mismatch",
			}

			winnerOpID := op.OpID
			loserOpID := current.LastOpID
			winnerDeviceID := deviceID
			loserDeviceID := current.LastDeviceID
			winnerClientUpdated := op.ClientUpdated
			loserClientUpdated := current.LastClientUpdated
			winnerSeq := seq
			loserSeq := current.LastSeq

			if !incomingWins {
				winnerOpID = current.LastOpID
				loserOpID = op.OpID
				winnerDeviceID = current.LastDeviceID
				loserDeviceID = deviceID
				winnerClientUpdated = current.LastClientUpdated
				loserClientUpdated = op.ClientUpdated
				winnerSeq = current.LastSeq
				loserSeq = seq
			}

			event.Winner = winnerOpID
			event.Loser = loserOpID
			event.WinnerDeviceID = winnerDeviceID
			event.LoserDeviceID = loserDeviceID

			if err := s.insertConflict(ctx, tx, op, event, winnerClientUpdated, loserClientUpdated, winnerSeq, loserSeq); err != nil {
				return pushResult{}, fmt.Errorf("insert conflict event %s: %w", op.OpID, err)
			}

			conflicts = append(conflicts, event)
		}
	}

	var latestCursor int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM sync_operations`).Scan(&latestCursor); err != nil {
		return pushResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return pushResult{}, err
	}

	return pushResult{
		Accepted:     accepted,
		Conflicts:    conflicts,
		ServerCursor: latestCursor,
	}, nil
}

func (s *PostgresStore) Pull(ctx context.Context, cursor int64, limit int) ([]syncOperation, int64, bool, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT seq, op_id, idempotency_key, entity_type, entity_id, op_type, COALESCE(payload, ''), COALESCE(client_updated_at, ''), COALESCE(base_version, 0)
		FROM sync_operations
		WHERE seq > $1
		  AND applied = TRUE
		ORDER BY seq ASC
		LIMIT $2
	`, cursor, limit+1)
	if err != nil {
		return nil, cursor, false, err
	}
	defer rows.Close()

	items := make([]syncOperation, 0, limit)
	nextCursor := cursor
	hasMore := false

	for rows.Next() {
		var (
			seq           int64
			opID          string
			idempotency   string
			entityType    string
			entityID      string
			opType        string
			payloadRaw    string
			clientUpdated string
			baseVersion   int
		)

		if err := rows.Scan(&seq, &opID, &idempotency, &entityType, &entityID, &opType, &payloadRaw, &clientUpdated, &baseVersion); err != nil {
			return nil, cursor, false, err
		}

		if len(items) >= limit {
			hasMore = true
			break
		}

		payload := []byte(strings.TrimSpace(payloadRaw))
		if len(payload) == 0 {
			payload = []byte("null")
		}

		items = append(items, syncOperation{
			OpID:           opID,
			IdempotencyKey: idempotency,
			EntityType:     entityType,
			EntityID:       entityID,
			OpType:         opType,
			Payload:        payload,
			ClientUpdated:  clientUpdated,
			BaseVersion:    baseVersion,
		})
		nextCursor = seq
	}

	if err := rows.Err(); err != nil {
		return nil, cursor, false, err
	}

	return items, nextCursor, hasMore, nil
}

func (s *PostgresStore) LatestCursor(ctx context.Context) (int64, error) {
	var cursor int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(seq), 0) FROM sync_operations`).Scan(&cursor)
	return cursor, err
}

func (s *PostgresStore) loadEntityStateForUpdate(ctx context.Context, tx *sql.Tx, entityType, entityID string) (entityStateSnapshot, bool, error) {
	var snapshot entityStateSnapshot
	err := tx.QueryRowContext(ctx, `
		SELECT version, last_seq, last_op_id, last_device_id, COALESCE(last_client_updated_at, '')
		FROM sync_entities
		WHERE entity_type = $1 AND entity_id = $2
		FOR UPDATE
	`, entityType, entityID).Scan(&snapshot.Version, &snapshot.LastSeq, &snapshot.LastOpID, &snapshot.LastDeviceID, &snapshot.LastClientUpdated)
	if err == sql.ErrNoRows {
		return entityStateSnapshot{}, false, nil
	}
	if err != nil {
		return entityStateSnapshot{}, false, err
	}
	return snapshot, true, nil
}

func (s *PostgresStore) upsertEntityState(ctx context.Context, tx *sql.Tx, op syncOperation, deviceID string, seq int64, version int) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sync_entities (
			entity_type,
			entity_id,
			version,
			last_seq,
			last_op_id,
			last_device_id,
			last_client_updated_at,
			op_type,
			payload,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT(entity_type, entity_id)
		DO UPDATE SET
			version = EXCLUDED.version,
			last_seq = EXCLUDED.last_seq,
			last_op_id = EXCLUDED.last_op_id,
			last_device_id = EXCLUDED.last_device_id,
			last_client_updated_at = EXCLUDED.last_client_updated_at,
			op_type = EXCLUDED.op_type,
			payload = EXCLUDED.payload,
			updated_at = NOW()
	`, op.EntityType, op.EntityID, version, seq, op.OpID, deviceID, op.ClientUpdated, op.OpType, string(op.Payload))
	return err
}

func (s *PostgresStore) insertConflict(ctx context.Context, tx *sql.Tx, op syncOperation, event conflictEvent, winnerClientUpdated, loserClientUpdated string, winnerSeq, loserSeq int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO sync_conflicts (
			entity_type,
			entity_id,
			reason,
			winner_op_id,
			loser_op_id,
			winner_device_id,
			loser_device_id,
			winner_client_updated_at,
			loser_client_updated_at,
			winner_seq,
			loser_seq,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW())
	`,
		op.EntityType,
		op.EntityID,
		event.Reason,
		event.Winner,
		event.Loser,
		event.WinnerDeviceID,
		event.LoserDeviceID,
		winnerClientUpdated,
		loserClientUpdated,
		winnerSeq,
		loserSeq,
	)
	return err
}
