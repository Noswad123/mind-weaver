package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := db.conn.Query(`
		SELECT src.id, COALESCE(src.title,''), src.path
		FROM links l
		JOIN notes src ON src.id = l.note_id
		WHERE l.type = 'internal'
		  AND l.resolved_note_id = ?
		ORDER BY COALESCE(src.title,''), src.id
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

func (db *NoteDb) ListGraphNodes(ctx context.Context) ([]GraphNodeRow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT n.id, COALESCE(ni.note_uid,''), COALESCE(n.title,''), n.path
		FROM notes n
		LEFT JOIN note_ids ni ON ni.note_id = n.id
		ORDER BY COALESCE(ni.note_uid,''), COALESCE(n.title,''), n.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []GraphNodeRow{}
	for rows.Next() {
		var r GraphNodeRow
		if err := rows.Scan(&r.ID, &r.UID, &r.Title, &r.Path); err != nil {
			return nil, err
		}
		r.Tags = splitCSVOrEmpty(db.graphStrings(ctx, `SELECT tag FROM tags WHERE note_id = ? ORDER BY tag`, r.ID))
		r.Domains = splitCSVOrEmpty(db.graphStrings(ctx, `SELECT domain FROM note_domains WHERE note_id = ? ORDER BY domain`, r.ID))
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) ListGraphEdges(ctx context.Context) ([]GraphEdgeRow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		WITH resolved_links AS (
			SELECT
				l.id,
				l.note_id,
				COALESCE(l.label,'') AS label,
				COALESCE(l.target,'') AS target,
				COALESCE(l.type,'internal') AS type,
				COALESCE(
					(SELECT n.id FROM notes n WHERE n.path = l.resolved_path LIMIT 1),
					(SELECT n.id FROM notes n WHERE n.path = l.resolved_path || '.md' LIMIT 1),
					(SELECT n.id FROM notes n WHERE n.path = l.resolved_path || '/hub.md' LIMIT 1),
					(SELECT ni.note_id FROM note_ids ni WHERE lower(ni.note_uid) = lower(trim(l.target)) ORDER BY ni.note_id LIMIT 1),
					(SELECT ni.note_id FROM note_ids ni WHERE lower(ni.note_uid) = lower(trim(l.resolved_path)) ORDER BY ni.note_id LIMIT 1)
				) AS target_note_id
			FROM links l
			WHERE l.type = 'internal'
		)
		SELECT rl.note_id, dst.id, rl.label, rl.target, rl.type
		FROM resolved_links rl
		JOIN notes dst ON dst.id = rl.target_note_id
		ORDER BY rl.note_id, dst.id, rl.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []GraphEdgeRow{}
	for rows.Next() {
		var r GraphEdgeRow
		if err := rows.Scan(&r.SourceID, &r.TargetID, &r.Label, &r.Target, &r.Kind); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) graphStrings(ctx context.Context, query string, noteID int) string {
	rows, err := db.conn.QueryContext(ctx, query, noteID)
	if err != nil {
		return ""
	}
	defer rows.Close()

	items := []string{}
	for rows.Next() {
		var item string
		if err := rows.Scan(&item); err != nil {
			continue
		}
		item = strings.TrimSpace(item)
		if item != "" {
			items = append(items, item)
		}
	}
	return strings.Join(items, ",")
}

func splitCSVOrEmpty(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
