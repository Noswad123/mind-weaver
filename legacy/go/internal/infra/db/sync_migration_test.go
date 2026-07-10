package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func createLegacySyncTables(t *testing.T, dbPath string) {
	t.Helper()

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS sync_outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			op_id TEXT NOT NULL UNIQUE,
			idempotency_key TEXT NOT NULL UNIQUE,
			entity_type TEXT NOT NULL,
			entity_key TEXT NOT NULL,
			op_type TEXT NOT NULL,
			payload TEXT,
			payload_hash TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempt_count INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			acked_at TEXT
		);
	`); err != nil {
		t.Fatalf("create legacy sync_outbox: %v", err)
	}

	if _, err := conn.Exec(`
		CREATE TABLE IF NOT EXISTS sync_todos (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'todo',
			priority TEXT NOT NULL DEFAULT 'medium',
			due_at TEXT,
			labels_json TEXT NOT NULL DEFAULT '[]',
			source TEXT NOT NULL DEFAULT 'sync',
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TEXT,
			payload TEXT NOT NULL DEFAULT '{}'
		);
	`); err != nil {
		t.Fatalf("create legacy sync_todos: %v", err)
	}

	if _, err := conn.Exec(`
		INSERT INTO sync_todos (id, title, status, priority, payload)
		VALUES ('legacy-1', 'Legacy Task', 'todo', 'medium', '{}')
	`); err != nil {
		t.Fatalf("insert legacy todo row: %v", err)
	}
}

func TestNewNoteDb_MigratesLegacySyncTables(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	createLegacySyncTables(t, dbPath)

	noteDB, err := NewNoteDb(dbPath, schemaPathForTests(t))
	if err != nil {
		t.Fatalf("open note db with migration: %v", err)
	}
	defer noteDB.Close()

	checks := []struct {
		table  string
		column string
	}{
		{table: "sync_outbox", column: "base_version"},
		{table: "sync_todos", column: "source_id"},
		{table: "sync_todos", column: "source_path"},
		{table: "sync_todos", column: "task_scope"},
		{table: "sync_todos", column: "task_area"},
		{table: "sync_todos", column: "todo_section"},
		{table: "sync_todos", column: "task_text"},
		{table: "sync_todos", column: "is_done"},
		{table: "sync_todos", column: "meta"},
		{table: "sync_todos", column: "task_order"},
		{table: "sync_todos", column: "line_number"},
	}

	for _, check := range checks {
		hasColumn, err := tableHasColumn(noteDB.conn, check.table, check.column)
		if err != nil {
			t.Fatalf("tableHasColumn(%s,%s): %v", check.table, check.column, err)
		}
		if !hasColumn {
			t.Fatalf("expected migrated column %s.%s", check.table, check.column)
		}
	}
}

func TestMigratedLegacySyncTodos_AcceptsTaskIndexUpsertShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "legacy-upsert.db")
	createLegacySyncTables(t, dbPath)

	noteDB, err := NewNoteDb(dbPath, schemaPathForTests(t))
	if err != nil {
		t.Fatalf("open note db with migration: %v", err)
	}
	defer noteDB.Close()

	sourcePath := "projects/hive/hub.md"
	taskScope := "project"
	taskArea := "Action"
	meta := "p3 e:m"
	line := 22

	err = noteDB.WithTx(ctx, func(tx *Tx) error {
		return tx.UpsertSyncTodo(SyncTodoUpsertParams{
			ID:          "todo-migrated-1",
			SourceID:    "hive-mind",
			SourcePath:  &sourcePath,
			TaskScope:   &taskScope,
			TaskArea:    &taskArea,
			TodoSection: "Inbox",
			TaskText:    "Validate migration safety",
			IsDone:      false,
			Meta:        &meta,
			TaskOrder:   2,
			LineNumber:  &line,
			Payload:     `{"id":"todo-migrated-1"}`,
		})
	})
	if err != nil {
		t.Fatalf("upsert task-index shape into migrated table: %v", err)
	}

	row, err := noteDB.GetSyncTodoByID(ctx, "todo-migrated-1")
	if err != nil {
		t.Fatalf("get migrated todo row: %v", err)
	}
	if row == nil {
		t.Fatalf("expected migrated todo row")
	}
	if row.TodoSection != "Inbox" {
		t.Fatalf("todo_section=%q want=Inbox", row.TodoSection)
	}
	if row.TaskText != "Validate migration safety" {
		t.Fatalf("task_text=%q want=%q", row.TaskText, "Validate migration safety")
	}
	if row.SourceID != "hive-mind" {
		t.Fatalf("source_id=%q want=hive-mind", row.SourceID)
	}
}
