package db

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Noswad123/mind-weaver/internal/parser"
	todoTypes "github.com/Noswad123/mind-weaver/internal/parser/todo"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath, schemaPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &DB{conn: conn}
	if err := db.createSchema(schemaPath); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) createSchema(schemaPath string) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	statements := strings.Split(string(schemaBytes), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.conn.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) UpsertNote(note parser.ParsedNote, path string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()

	timestamp := time.Now().Format(time.RFC3339)
	_, err = tx.Exec(`INSERT INTO notes (path, title, content, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET title=excluded.title, content=excluded.content, updated_at=excluded.updated_at`,
		path, note.Title, note.Content, timestamp)
	if err != nil {
		return err
	}

	var noteID int
	err = tx.QueryRow(`SELECT id FROM notes WHERE path = ?`, path).Scan(&noteID)
	if err != nil {
		return err
	}

	tx.Exec(`DELETE FROM tags WHERE note_id = ?`, noteID)
	tx.Exec(`DELETE FROM links WHERE note_id = ?`, noteID)
	tx.Exec(`DELETE FROM todos WHERE note_id = ?`, noteID)
	tx.Exec(`DELETE FROM task_groups WHERE note_id = ?`, noteID)

	groupIDMap := map[string]int{}
	for _, tag := range note.Tags {
		tx.Exec(`INSERT INTO tags (note_id, tag) VALUES (?, ?)`, noteID, tag)
	}
	for _, t := range note.Todos {
		if t.IsGroup && t.Task != nil {
			var derivedID *int
			if t.DerivedGroup != nil {
				if id, ok := groupIDMap[*t.DerivedGroup]; ok {
					derivedID = &id
				}
			}
			res, err := tx.Exec(`INSERT INTO task_groups (note_id, name, level, derived_group_id, status, raw_status, line_number)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				noteID, *t.Task, t.Level, derivedID, t.Status, t.RawStatus, t.Line)
			if err != nil {
				continue
			}
			id, _ := res.LastInsertId()
			groupIDMap[*t.Task] = int(id)
		}
	}
	for _, t := range note.Todos {
		if !t.IsGroup && t.Task != nil {
			groupID, ok := groupIDMap[t.Group]
			if !ok {
				continue
			}
			tx.Exec(`INSERT INTO todos (note_id, task_group_id, task, status, raw_status, depth, line_number)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				noteID, groupID, *t.Task, t.Status, t.RawStatus, t.Depth, t.Line)
		}
	}
	for _, l := range note.Links {
		resolved := ""
		if l.ResolvedPath != nil {
			resolved = *l.ResolvedPath
		}
		tx.Exec(`INSERT INTO links (note_id, label, target, type, resolved_path)
			VALUES (?, ?, ?, ?, ?)`,
			noteID, l.Label, l.Target, l.Type, resolved)
	}

	return nil
} // Remaining methods will be updated similarly to use (db *DB)

