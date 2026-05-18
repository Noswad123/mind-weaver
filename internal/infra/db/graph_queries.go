package db

import (
	"fmt"
	"database/sql"
)

func (db *NoteDb) GetOutlinksByNoteID(noteID, limit, offset int) ([]NoteLiteRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := db.conn.Query(`
		SELECT n.id, COALESCE(n.title,''), n.path
		FROM links l
		JOIN notes n ON n.path = l.resolved_path
		WHERE l.note_id = ?
		  AND l.type = 'internal'
		  AND COALESCE(l.resolved_path,'') <> ''
		ORDER BY COALESCE(n.title,''), n.id
		LIMIT ? OFFSET ?
	`, noteID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NoteLiteRow
	for rows.Next() {
		var r NoteLiteRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) GetBacklinksByNoteID(noteID, limit, offset int) ([]NoteLiteRow, error) {
	if limit <= 0 { limit = 50 }
	if offset < 0 { offset = 0 }

	rows, err := db.conn.Query(`
		SELECT src.id, COALESCE(src.title,''), src.path
		FROM links l
		JOIN notes src ON src.id = l.note_id
		WHERE l.type = 'internal'
		  AND l.resolved_note_id = ?
		ORDER BY COALESCE(src.title,''), src.id
		LIMIT ? OFFSET ?
	`, noteID, limit, offset)
	if err != nil { return nil, err }
	defer rows.Close()

	var out []NoteLiteRow
	for rows.Next() {
		var r NoteLiteRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path); err != nil { return nil, err }
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) GetNoteLiteByID(id int) (NoteLiteRow, error) {
	var r NoteLiteRow
	err := db.conn.QueryRow(`
		SELECT id, COALESCE(title,''), path
		FROM notes
		WHERE id = ?
	`, id).Scan(&r.ID, &r.Title, &r.Path)
	if err != nil {
		return NoteLiteRow{}, err
	}
	return r, nil
}

func (db *NoteDb) GetNoteContentByID(id int) (string, error) {
	var c sql.NullString
	err := db.conn.QueryRow(`SELECT content FROM notes WHERE id = ?`, id).Scan(&c)
	if err != nil {
		return "", err
	}
	if !c.Valid {
		return "", nil
	}
	return c.String, nil
}

func (db *NoteDb) EnsureGraphIndexes() error {
	_, err := db.conn.Exec(`
		CREATE INDEX IF NOT EXISTS idx_links_note_id ON links(note_id);
		CREATE INDEX IF NOT EXISTS idx_links_resolved_path ON links(resolved_path);
		CREATE INDEX IF NOT EXISTS idx_notes_path ON notes(path);
	`)
	if err != nil {
		return fmt.Errorf("ensure graph indexes: %w", err)
	}
	return nil
}
