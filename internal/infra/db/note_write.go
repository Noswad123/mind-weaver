package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (t *Tx) QueryRow(query string, args ...any) *sql.Row { return t.tx.QueryRow(query, args...) }

func (t *Tx) UpsertNoteRow(path, title, content string, updatedAt time.Time) (int, error) {
	_, err := t.tx.Exec(`
		INSERT INTO notes (path, title, content, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title=excluded.title,
			content=excluded.content,
			updated_at=excluded.updated_at
	`, path, title, content, updatedAt.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}

	var noteID int
	if err := t.tx.QueryRow(`SELECT id FROM notes WHERE path = ?`, path).Scan(&noteID); err != nil {
		return 0, err
	}
	return noteID, nil
}

func (t *Tx) ClearNoteChildren(noteID int) error {
	if _, err := t.tx.Exec(`DELETE FROM note_domains WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := t.tx.Exec(`DELETE FROM tags WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := t.tx.Exec(`DELETE FROM links WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := t.tx.Exec(`DELETE FROM todos WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := t.tx.Exec(`DELETE FROM task_groups WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	return nil
}

func (t *Tx) InsertDomains(noteID int, domains []string) error {
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		if _, err := t.tx.Exec(`INSERT OR IGNORE INTO note_domains (note_id, domain) VALUES (?, ?)`, noteID, domain); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tx) InsertTags(noteID int, tags []string) error {
	for _, tag := range tags {
		if _, err := t.tx.Exec(`INSERT INTO tags (note_id, tag) VALUES (?, ?)`, noteID, tag); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tx) InsertLinks(noteID int, links []LinkRow) error {
	for _, l := range links {
		typ := strings.TrimSpace(l.Type)
		if typ == "" {
			typ = "internal"
		}
		if typ != "internal" && typ != "external" {
			return fmt.Errorf("invalid link type %q", l.Type)
		}

		_, err := t.tx.Exec(`
			INSERT INTO links (note_id, label, target, type, resolved_path)
			VALUES (?, ?, ?, ?, ?)
		`, noteID, l.Label, l.Target, typ, l.ResolvedPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tx) InsertTaskGroup(noteID int, name string, level int, derivedGroupID *int, status, rawStatus string, line int) (int, error) {
	res, err := t.tx.Exec(`
		INSERT INTO task_groups (note_id, name, level, derived_group_id, status, raw_status, line_number)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, noteID, name, level, derivedGroupID, status, rawStatus, line)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (t *Tx) InsertTodo(noteID int, groupID int, task, status, rawStatus string, depth, line int) error {
	_, err := t.tx.Exec(`
		INSERT INTO todos (note_id, task_group_id, task, status, raw_status, depth, line_number)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, noteID, groupID, task, status, rawStatus, depth, line)
	return err
}

func (t *Tx) DeleteNoteByPath(path string) error {
	_, err := t.tx.Exec(`DELETE FROM notes WHERE path = ?`, path)
	return err
}
