package db

import (
	"database/sql"
	"fmt"
	"strings"
)

type RegisteredNoteID struct {
	NoteID *int
	UID    string
	Path   string
	IsHub  bool
}

type RegistryConflict struct {
	NoteID *int
	UID    string
	Path   string
	IsHub  bool
	Reason string // DUPLICATE_ID | MISSING_HUB_ID | NOTE_NOT_IN_DB
}

func (db *NoteDb) NoteIDByPath(path string) (*int, error) {
	var id int
	err := db.conn.QueryRow(`SELECT id FROM notes WHERE path = ?`, path).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (db *NoteDb) ReplaceRegistry(entries []RegisteredNoteID, conflicts []RegistryConflict) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM note_ids`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM note_id_conflicts`); err != nil {
		return err
	}

	for _, e := range entries {
		if e.NoteID == nil {
			continue
		}
		uid := strings.TrimSpace(e.UID)
		if uid == "" {
			continue
		}

		_, err := tx.Exec(`
			INSERT INTO note_ids (note_id, note_uid, path, is_hub, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, *e.NoteID, uid, e.Path, boolToInt(e.IsHub))
		if err != nil {
			return fmt.Errorf("insert note_ids (%s): %w", e.Path, err)
		}
	}

	for _, c := range conflicts {
		reason := normalizeReason(c.Reason)

		noteIDVal := any(nil)
		if c.NoteID != nil {
			noteIDVal = *c.NoteID
		}

		_, err := tx.Exec(`
			INSERT INTO note_id_conflicts (note_id, note_uid, path, is_hub, reason, detected_at)
			VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		`, noteIDVal, nullIfEmpty(c.UID), c.Path, boolToInt(c.IsHub), reason)
		if err != nil {
			return fmt.Errorf("insert note_id_conflicts (%s): %w", c.Path, err)
		}
	}

	return tx.Commit()
}

func normalizeReason(r string) string {
	r = strings.TrimSpace(strings.ToUpper(r))
	switch r {
	case "DUPLICATE_ID", "MISSING_HUB_ID", "NOTE_NOT_IN_DB":
		return r
	default:
		// safest default: treat unknown as DUPLICATE_ID so it doesn't violate CHECK (unless you add NOTE_NOT_IN_DB)
		return "DUPLICATE_ID"
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

type NoteIDRow struct {
	NoteID    int
	UID       string
	Path      string
	IsHub     bool
	UpdatedAt string
}

type NoteIDConflictRow struct {
	NoteID     *int
	UID        *string
	Path       string
	IsHub      bool
	Reason     string
	DetectedAt string
}

func (db *NoteDb) GetNoteIDRegistry() ([]NoteIDRow, error) {
	rows, err := db.conn.Query(`
		SELECT note_id, note_uid, path, is_hub, updated_at
		FROM note_ids
		ORDER BY note_uid
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []NoteIDRow{}
	for rows.Next() {
		var r NoteIDRow
		var isHub int
		if err := rows.Scan(&r.NoteID, &r.UID, &r.Path, &isHub, &r.UpdatedAt); err != nil {
			continue
		}
		r.IsHub = isHub == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) GetNoteIDConflicts() ([]NoteIDConflictRow, error) {
	rows, err := db.conn.Query(`
		SELECT note_id, note_uid, path, is_hub, reason, detected_at
		FROM note_id_conflicts
		ORDER BY reason, note_uid, path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []NoteIDConflictRow
	for rows.Next() {
		var (
			noteID sql.NullInt64
			uid    sql.NullString
			path   string
			isHub  int
			reason string
			dt     string
		)
		if err := rows.Scan(&noteID, &uid, &path, &isHub, &reason, &dt); err != nil {
			return nil, err
		}

		var nid *int
		if noteID.Valid {
			v := int(noteID.Int64)
			nid = &v
		}
		var puid *string
		if uid.Valid {
			v := uid.String
			puid = &v
		}

		out = append(out, NoteIDConflictRow{
			NoteID: nid, UID: puid, Path: path, IsHub: isHub == 1,
			Reason: reason, DetectedAt: dt,
		})
	}
	return out, rows.Err()
}
