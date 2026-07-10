package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func normalizeSyncEntityKey(entityType syncops.EntityType, entityKey string) (syncops.EntityType, string, error) {
	entityType = syncops.EntityType(strings.TrimSpace(string(entityType)))
	entityKey = strings.TrimSpace(entityKey)

	if entityType != syncops.EntityNote && entityType != syncops.EntityTodo {
		return "", "", errors.New("entity type must be note or todo")
	}
	if entityKey == "" {
		return "", "", errors.New("entity key is required")
	}

	return entityType, entityKey, nil
}

func (t *Tx) ResolveSyncEntityVersion(entityType syncops.EntityType, entityKey string) (int, error) {
	entityType, entityKey, err := normalizeSyncEntityKey(entityType, entityKey)
	if err != nil {
		return 0, err
	}

	var version int
	err = t.tx.QueryRow(`
		SELECT version
		FROM sync_entity_versions
		WHERE entity_type = ?
		  AND entity_key = ?
	`, entityType, entityKey).Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	if version < 0 {
		return 0, nil
	}

	return version, nil
}

func (t *Tx) SetSyncEntityVersion(entityType syncops.EntityType, entityKey string, version int) error {
	entityType, entityKey, err := normalizeSyncEntityKey(entityType, entityKey)
	if err != nil {
		return err
	}
	if version < 0 {
		return errors.New("version must be >= 0")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = t.tx.Exec(`
		INSERT INTO sync_entity_versions (entity_type, entity_key, version, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_key) DO UPDATE SET
			version = excluded.version,
			updated_at = excluded.updated_at
	`, entityType, entityKey, version, now)
	return err
}

func (t *Tx) IncrementSyncEntityVersion(entityType syncops.EntityType, entityKey string) (int, error) {
	current, err := t.ResolveSyncEntityVersion(entityType, entityKey)
	if err != nil {
		return 0, err
	}
	next := current + 1
	if err := t.SetSyncEntityVersion(entityType, entityKey, next); err != nil {
		return 0, err
	}
	return next, nil
}

func (db *NoteDb) GetSyncEntityVersion(ctx context.Context, entityType syncops.EntityType, entityKey string) (*int, error) {
	entityType, entityKey, err := normalizeSyncEntityKey(entityType, entityKey)
	if err != nil {
		return nil, err
	}

	var version int
	err = db.conn.QueryRowContext(ctx, `
		SELECT version
		FROM sync_entity_versions
		WHERE entity_type = ?
		  AND entity_key = ?
	`, entityType, entityKey).Scan(&version)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if version < 0 {
		v := 0
		return &v, nil
	}

	return &version, nil
}
