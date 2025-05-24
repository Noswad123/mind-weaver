package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
	"context"

	_ "github.com/mattn/go-sqlite3"

	"github.com/Noswad123/mind-weaver/internal/parser"
	todoTypes "github.com/Noswad123/mind-weaver/internal/parser/todo"
)

type Query struct {
	Name string
	SQL  string
}

type NoteDb struct {
	conn *sql.DB
}

func NewNoteDb(dbPath, schemaPath string) (*NoteDb, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &NoteDb{conn: conn}
	if err := db.createSchema(schemaPath); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *NoteDb) Close() error {
	return db.conn.Close()
}

func (db *NoteDb) createSchema(schemaPath string) error {
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

func (db *NoteDb) UpsertNote(note parser.ParsedNote, path string) error {
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
}


func (db *NoteDb) GetNoteByID(id int) (parser.ParsedNote, error) {
	row := db.conn.QueryRow(`SELECT id, title, path, content FROM notes WHERE id = ?`, id)

	var noteID int
	var title, path, content string
	if err := row.Scan(&noteID, &title, &path, &content); err != nil {
		return parser.ParsedNote{}, err
	}

	tags, _ := db.getTagsForNote(noteID)
	todos, _ := db.getTodosForNote(noteID)
	links, _ := db.getLinksForNote(noteID)

	return parser.ParsedNote{
		Title:   title,
		Content: content,
		Tags:    tags,
		Todos:   todos,
		Links:   links,
	}, nil
}

func (db *NoteDb) SearchNotesByName(input string) ([]parser.ParsedNote, error) {
	query := `
	SELECT DISTINCT id, title, path, content FROM notes
	WHERE LOWER(title) LIKE '%' || LOWER(?) || '%'`

	rows, err := db.conn.Query(query, input)
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
		tags, _ := db.getTagsForNote(noteID)
		todos, _ := db.getTodosForNote(noteID)
		links, _ := db.getLinksForNote(noteID)

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

func (db *NoteDb) GetNotesByTags(tags []string) ([]parser.ParsedNote, error) {
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

	rows, err := db.conn.Query(query, args...)
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

		tags, _ := db.getTagsForNote(noteID)
		todos, _ := db.getTodosForNote(noteID)
		links, _ := db.getLinksForNote(noteID)

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

func (db *NoteDb) GetAllNotes() ([]parser.ParsedNote, error) {
	rows, err := db.conn.Query(`SELECT title, path, content FROM notes`)
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

func (db *NoteDb)getTagsForNote(noteID int) ([]string, error) {
	rows, err := db.conn.Query(`SELECT tag FROM tags WHERE note_id = ?`, noteID)
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

func (db *NoteDb) getLinksForNote(noteID int) ([]parser.Link, error) {
	rows, err := db.conn.Query(`
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

func (db *NoteDb) getTodosForNote(noteID int) ([]todoTypes.Todo, error) {
	groupRows, err := db.conn.Query(`
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

	todoRows, err := db.conn.Query(`
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

func (db *NoteDb) ExecuteSQL(sqlStr string) (string, error) {
	sqlStr = strings.TrimSpace(sqlStr)
	if sqlStr == "" {
		return "", nil
	}

	ctx := context.Background()
	rows, err := db.conn.QueryContext(ctx, sqlStr)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var output strings.Builder
	output.WriteString(strings.Join(cols, " | ") + "\n")
	output.WriteString(strings.Repeat("-", 80) + "\n")

	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}

	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		for i, v := range vals {
			if i > 0 {
				output.WriteString(" | ")
			}
			output.WriteString(fmt.Sprintf("%v", v))
		}
		output.WriteString("\n")
	}

	return output.String(), nil
}

func (db *NoteDb) LoadSavedQueries() ([]Query, error) {
	rows, err := db.conn.Query(`SELECT name, sql FROM saved_queries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var queries []Query
	for rows.Next() {
		var q Query
		if err := rows.Scan(&q.Name, &q.SQL); err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, nil
}

func (db *NoteDb) GetWorkspaceNotePaths() ([]string, error) {
	rows, err := db.conn.Query(`SELECT path FROM notes WHERE path LIKE '%/index.norg'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		if path == "index.norg" {
			continue
		}
		paths = append(paths, path)
	}
	return paths, nil
}
