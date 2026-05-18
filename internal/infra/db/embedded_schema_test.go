package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewNoteDbUsesEmbeddedSchemaWhenPathIsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "note.db")
	noteDB, err := NewNoteDb(dbPath, "")
	if err != nil {
		t.Fatalf("NewNoteDb() error = %v", err)
	}
	defer noteDB.Close()

	out, err := noteDB.ExecuteSQL(context.Background(), `SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'notes'`)
	if err != nil {
		t.Fatalf("query schema table: %v", err)
	}
	if out == "name\n" {
		t.Fatalf("notes table was not created")
	}
}

func TestNewCommandDbUsesEmbeddedSchemaWhenPathIsEmpty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "command.db")
	commandDB, err := NewCommandDb(dbPath, "")
	if err != nil {
		t.Fatalf("NewCommandDb() error = %v", err)
	}
	defer commandDB.Close()

	if _, err := commandDB.conn.Exec(`INSERT INTO tools (name, description) VALUES ('git', 'version control')`); err != nil {
		t.Fatalf("insert into embedded command schema: %v", err)
	}
}
