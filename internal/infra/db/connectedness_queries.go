package db

import (
	"fmt"
	"time"
)

func (db *NoteDb) RecomputeConnectedness() error {
	now := time.Now().Format(time.RFC3339)

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM note_connectedness`); err != nil {
		return fmt.Errorf("delete note_connectedness: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO note_connectedness(note_id, in_degree, out_degree, updated_at)
		SELECT n.id,
		  (SELECT COUNT(*) FROM links l WHERE l.resolved_note_id = n.id) AS in_degree,
		  (SELECT COUNT(*) FROM links l WHERE l.note_id = n.id AND l.resolved_note_id IS NOT NULL) AS out_degree,
		  ?
		FROM notes n
	`, now); err != nil {
		return fmt.Errorf("recompute note_connectedness: %w", err)
	}

	return tx.Commit()
}

func (db *NoteDb) ListNotesByConnectedness(limit, offset int) ([]NoteDegreeRow, error) {
	if limit <= 0 { limit = 50 }
	if offset < 0 { offset = 0 }

	rows, err := db.conn.Query(`
		SELECT n.id, COALESCE(n.title,''), n.path,
			   COALESCE(c.in_degree,0), COALESCE(c.out_degree,0)
		FROM notes n
		LEFT JOIN note_connectedness c ON c.note_id = n.id
		ORDER BY (COALESCE(c.in_degree,0) + COALESCE(c.out_degree,0)) DESC,
				 COALESCE(n.updated_at,'') DESC,
				 n.id DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil { return nil, err }
	defer rows.Close()

	var out []NoteDegreeRow
	for rows.Next() {
		var r NoteDegreeRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.In, &r.Out); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) SearchNotesByConnectedness(q string, limit, offset int) ([]NoteDegreeRow, error) {
	if limit <= 0 { limit = 50 }
	if offset < 0 { offset = 0 }

	rows, err := db.conn.Query(`
		SELECT n.id, COALESCE(n.title,''), n.path,
			   COALESCE(c.in_degree,0), COALESCE(c.out_degree,0)
		FROM notes n
		LEFT JOIN note_connectedness c ON c.note_id = n.id
		WHERE LOWER(COALESCE(n.title,'')) LIKE '%'||LOWER(?)||'%'
		   OR LOWER(n.path) LIKE '%'||LOWER(?)||'%'
		ORDER BY (COALESCE(c.in_degree,0) + COALESCE(c.out_degree,0)) DESC,
				 COALESCE(n.updated_at,'') DESC,
				 n.id DESC
		LIMIT ? OFFSET ?
	`, q, q, limit, offset)
	if err != nil { return nil, err }
	defer rows.Close()

	var out []NoteDegreeRow
	for rows.Next() {
		var r NoteDegreeRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.In, &r.Out); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
