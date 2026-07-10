package db

import (
	"context"
	"database/sql"
)

type Tx struct {
	tx *sql.Tx
}

func (db *NoteDb) WithTx(ctx context.Context, fn func(*Tx) error) error {
	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(&Tx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}
