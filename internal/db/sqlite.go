package db

import (
	"database/sql"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/Noswad123/mind-weaver/internal/parser"
	todoTypes "github.com/Noswad123/mind-weaver/internal/parser/todo"
)

var Conn *sql.DB

func InitDBWithPaths(dbPath, schemaPath string) {
	var err error
	Conn, err = sql.Open("sqlite3", dbPath)
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
		if _, err := Conn.Exec(trimmed); err != nil {
			log.Fatalf("❌ Schema migration error: %v\n⚠️ Statement: %s", err, trimmed)
		}
	}
}

func UpsertNote(note parser.ParsedNote, path string) {
	tx, err := Conn.Begin()
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

func GetNoteByID(id int) (parser.ParsedNote, error) {
	row := Conn.QueryRow(`SELECT id, title, path, content FROM notes WHERE id = ?`, id)

	var noteID int
	var title, path, content string
	if err := row.Scan(&noteID, &title, &path, &content); err != nil {
		return parser.ParsedNote{}, err
	}

	tags, _ := getTagsForNote(noteID)
	todos, _ := getTodosForNote(noteID)
	links, _ := getLinksForNote(noteID)

	return parser.ParsedNote{
		Title:   title,
		Content: content,
		Tags:    tags,
		Todos:   todos,
		Links:   links,
	}, nil
}

func SearchNotesByName(input string) ([]parser.ParsedNote, error) {
	query := `
	SELECT DISTINCT id, title, path, content FROM notes
	WHERE LOWER(title) LIKE '%' || LOWER(?) || '%'`

	rows, err := Conn.Query(query, input)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []parser.ParsedNote
	for rows.Next() {
		var noteID int
		var title, path, content string
		if err := rows.Scan(noteID, &title, &path, &content); err != nil {
			continue
		}
		tags, _ := getTagsForNote(noteID)
		todos, _ := getTodosForNote(noteID)
		links, _ := getLinksForNote(noteID)

		results = append(results, parser.ParsedNote{
			Title:   title,
			Content: content,
			Tags:    tags,
			Todos:   todos,
			Links:   links,
		})
	}

	return results, rows.Err()
}

func GetNotesByTags(tags []string) ([]parser.ParsedNote, error) {
	placeholders := strings.Repeat("?,", len(tags))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	query := `
	SELECT DISTINCT n.title, n.path, n.content
	FROM notes n
	JOIN tags t ON n.id = t.note_id
	WHERE t.tag IN (` + placeholders + `)`

	args := make([]any, len(tags))
	for i, tag := range tags {
		args[i] = tag
	}

	rows, err := Conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []parser.ParsedNote
	for rows.Next() {
		var noteID int
		var title, path, content string
		if err := rows.Scan(&noteID, &title, &path, &content); err != nil {
			continue
		}

		tags, _ := getTagsForNote(noteID)
		todos, _ := getTodosForNote(noteID)
		links, _ := getLinksForNote(noteID)

		results = append(results, parser.ParsedNote{
			Title:   title,
			Content: content,
			Tags:    tags,
			Todos:   todos,
			Links:   links,
		})
	}

	return results, rows.Err()
}

func GetAllNotes() ([]parser.ParsedNote, error) {
	rows, err := Conn.Query(`SELECT title, path, content FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []parser.ParsedNote
	for rows.Next() {
		var title, path, content string
		if err := rows.Scan(&title, &path, &content); err != nil {
			continue
		}
		results = append(results, parser.ParsedNote{
			Title:   title,
			Content: content,
		})
	}

	return results, rows.Err()
}

func getTagsForNote(noteID int) ([]string, error) {
	rows, err := Conn.Query(`SELECT tag FROM tags WHERE note_id = ?`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func getLinksForNote(noteID int) ([]parser.Link, error) {
	rows, err := Conn.Query(`
		SELECT label, target, type, resolved_path
		FROM links WHERE note_id = ?`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var links []parser.Link
	for rows.Next() {
		var l parser.Link
		var resolved sql.NullString
		if err := rows.Scan(&l.Label, &l.Target, &l.Type, &resolved); err != nil {
			continue
		}
		if resolved.Valid {
			l.ResolvedPath = &resolved.String
		}
		links = append(links, l)
	}
	return links, nil
}

func getTodosForNote(noteID int) ([]todoTypes.Todo, error) {
	// First fetch groups
	groupRows, err := Conn.Query(`
		SELECT id, name, level, derived_group_id, status, raw_status, line_number
		FROM task_groups WHERE note_id = ?`, noteID)
	if err != nil {
		return nil, err
	}
	defer groupRows.Close()

	groupMap := map[int]string{}
	var todos []todoTypes.Todo

	for groupRows.Next() {
		var id, level, line int
		var name, status, raw string
		var derived sql.NullInt64
		if err := groupRows.Scan(&id, &name, &level, &derived, &status, &raw, &line); err != nil {
			continue
		}
		groupMap[id] = name
		t := todoTypes.Todo{
			IsGroup:      true,
			Task:         &name,
			Level:        level,
			Status:       status,
			RawStatus:    raw,
			Line:         line,
		}
		if derived.Valid {
			derivedName := groupMap[int(derived.Int64)]
			t.DerivedGroup = &derivedName
		}
		todos = append(todos, t)
	}

	// Then fetch regular todos
	todoRows, err := Conn.Query(`
		SELECT task_group_id, task, status, raw_status, depth, line_number
		FROM todos WHERE note_id = ?`, noteID)
	if err != nil {
		return nil, err
	}
	defer todoRows.Close()

	for todoRows.Next() {
		var groupID, depth, line int
		var task, status, raw string
		if err := todoRows.Scan(&groupID, &task, &status, &raw, &depth, &line); err != nil {
			continue
		}
		groupName := groupMap[groupID]
		t := todoTypes.Todo{
			IsGroup:   false,
			Task:      &task,
			Group:     groupName,
			Status:    status,
			RawStatus: raw,
			Depth:     depth,
			Line:      line,
		}
		todos = append(todos, t)
	}

	return todos, nil
}