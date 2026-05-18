package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type NoteDb struct {
	conn *sql.DB
}

func (db *NoteDb) createSchema(schemaPath string) error {
	schema, err := readSchema(schemaPath, EmbeddedSchemaPath, embeddedNoteSchema)
	if err != nil {
		return err
	}

	schema = strings.TrimSpace(schema)
	if schema == "" {
		return fmt.Errorf("schema is empty: %s", schemaPath)
	}

	if _, err := db.conn.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return err
	}

	if err := db.migrateRegistryTablesIfNeeded(); err != nil {
		return fmt.Errorf("migrate registry tables: %w", err)
	}

	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("apply schema %s: %w", schemaPath, err)
	}

	if err := db.migrateSyncTablesIfNeeded(); err != nil {
		return fmt.Errorf("migrate sync tables: %w", err)
	}

	return nil
}

func (db *NoteDb) migrateRegistryTablesIfNeeded() error {
	noteIDsLegacy, err := tableHasColumn(db.conn, "note_ids", "is_index")
	if err != nil {
		return err
	}
	conflictsLegacy, err := tableHasColumn(db.conn, "note_id_conflicts", "is_index")
	if err != nil {
		return err
	}
	if !noteIDsLegacy && !conflictsLegacy {
		return nil
	}

	if _, err := db.conn.Exec(`DROP TABLE IF EXISTS note_id_conflicts`); err != nil {
		return err
	}
	if _, err := db.conn.Exec(`DROP TABLE IF EXISTS note_ids`); err != nil {
		return err
	}

	return nil
}

func (db *NoteDb) migrateSyncTablesIfNeeded() error {
	hasBaseVersion, err := tableHasColumn(db.conn, "sync_outbox", "base_version")
	if err != nil {
		return err
	}
	if !hasBaseVersion {
		if _, err := db.conn.Exec(`ALTER TABLE sync_outbox ADD COLUMN base_version INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
	}

	if err := ensureSyncTodosColumns(db.conn); err != nil {
		return err
	}

	return nil
}

func ensureSyncTodosColumns(conn *sql.DB) error {
	type columnSpec struct {
		Name       string
		Definition string
	}

	required := []columnSpec{
		{Name: "source_id", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "source_path", Definition: "TEXT"},
		{Name: "task_scope", Definition: "TEXT"},
		{Name: "task_area", Definition: "TEXT"},
		{Name: "todo_section", Definition: "TEXT NOT NULL DEFAULT 'Inbox'"},
		{Name: "task_text", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "is_done", Definition: "INTEGER NOT NULL DEFAULT 0"},
		{Name: "meta", Definition: "TEXT"},
		{Name: "task_order", Definition: "INTEGER NOT NULL DEFAULT 0"},
		{Name: "line_number", Definition: "INTEGER"},
	}

	for _, column := range required {
		hasColumn, err := tableHasColumn(conn, "sync_todos", column.Name)
		if err != nil {
			return err
		}
		if hasColumn {
			continue
		}

		if _, err := conn.Exec(`ALTER TABLE sync_todos ADD COLUMN ` + column.Name + ` ` + column.Definition); err != nil {
			return err
		}
	}

	if _, err := conn.Exec(`CREATE INDEX IF NOT EXISTS idx_sync_todos_updated ON sync_todos(updated_at DESC)`); err != nil {
		return err
	}

	return nil
}

func tableHasColumn(conn *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := conn.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			typeName   string
			notNull    int
			defaultV   sql.NullString
			primaryKey int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultV, &primaryKey); err != nil {
			return false, err
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func NewNoteDb(dbPath, schemaPath string) (*NoteDb, error) {
	if dir := filepath.Dir(dbPath); dir != "." && strings.TrimSpace(dir) != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

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

func (db *NoteDb) ExecuteSQL(ctx context.Context, query string) (string, error) {
	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString(strings.Join(cols, "\t"))
	buf.WriteString("\n")

	// prepare scan targets
	raw := make([]any, len(cols))
	dest := make([]any, len(cols))
	for i := range raw {
		dest[i] = &raw[i]
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return "", err
		}
		for i, v := range raw {
			if i > 0 {
				buf.WriteString("\t")
			}
			buf.WriteString(formatCell(v))
		}
		buf.WriteString("\n")
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func formatCell(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return string(t)
	case string:
		return t
	default:
		return fmt.Sprintf("%v", t)
	}
}
