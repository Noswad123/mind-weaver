package db

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

var conn *sql.DB

func InitDBWithPaths(dbPath, schemaPath string) {
	var err error
	conn, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("❌ Failed to open DB: %v", err)
	}
	createSchema(schemaPath)
}

func createSchema(schemaPath string) {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		log.Fatalf("❌ Failed to read schema.sql at %s: %v", schemaPath, err)
	}

	statements := strings.Split(string(schemaBytes), ";")
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		if _, err := conn.Exec(trimmed); err != nil {
			log.Fatalf("❌ Schema migration error: %v\n⚠️ Statement: %s", err, trimmed)
		}
	}
}

func UpsertNote(note parser.ParsedNote, path string) {
	tx, err := conn.Begin()
	if err != nil {
		log.Println("Failed to start transaction:", err)
		return
	}
	defer tx.Commit()

	timestamp := time.Now().Format(time.RFC3339)
	_, err = tx.Exec(`INSERT INTO notes (path, title, content, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET title=excluded.title, content=excluded.content, updated_at=excluded.updated_at`,
		path, note.Title, note.Content, timestamp)
	if err != nil {
		log.Println("Insert/update note failed:", err)
		return
	}

	var noteID int
	err = tx.QueryRow(`SELECT id FROM notes WHERE path = ?`, path).Scan(&noteID)
	if err != nil {
		log.Println("Failed to fetch note_id:", err)
		return
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
				log.Println("Insert task group failed:", err)
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
				log.Printf("⚠️ Skipping todo: no task group found for %s\n", t.Group)
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
}
