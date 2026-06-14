package db

import (
	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

func (db *NoteDb) getTagsForNote(noteID shared.ID) ([]string, error) {
	rows, err := db.conn.Query(`SELECT tag FROM tags WHERE note_id = ? ORDER BY tag`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tags := []string{}
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (db *NoteDb) getDomainsForNote(noteID shared.ID) ([]string, error) {
	rows, err := db.conn.Query(`SELECT domain FROM note_domains WHERE note_id = ? ORDER BY domain`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	domains := []string{}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

func (db *NoteDb) getLinksForNote(noteID shared.ID) ([]LinkRow, error) {
	rows, err := db.conn.Query(`
		SELECT COALESCE(label, ''), COALESCE(target, ''), COALESCE(type, ''), COALESCE(resolved_path, '')
		FROM links
		WHERE note_id = ?
		ORDER BY id ASC
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LinkRow{}
	for rows.Next() {
		var r LinkRow
		if err := rows.Scan(&r.Label, &r.Target, &r.Type, &r.ResolvedPath); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
