package db

import (
	"database/sql"
	"strings"
)

func (t *Tx) ResolveNoteIDByPath(path string) (*int, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}

	var id int
	err := t.tx.QueryRow(`SELECT id FROM notes WHERE path = ?`, path).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

