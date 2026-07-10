package db

import (
	"context"
	"testing"
)

func TestUpsertSyncTodo_RoundTripTaskIndexShape(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	sourcePath := "projects/hive/hub.md"
	taskScope := "project"
	taskArea := "Action"
	meta := "p2 e:m"
	line := 42
	updated := "2026-04-05T00:00:00Z"

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		return tx.UpsertSyncTodo(SyncTodoUpsertParams{
			ID:          "todo-1",
			SourceID:    "hive-mind",
			SourcePath:  &sourcePath,
			TaskScope:   &taskScope,
			TaskArea:    &taskArea,
			TodoSection: "Next",
			TaskText:    "Call electrician",
			IsDone:      false,
			Meta:        &meta,
			TaskOrder:   3,
			LineNumber:  &line,
			UpdatedAt:   &updated,
			Payload:     `{"id":"todo-1","todo_section":"Next","text":"Call electrician"}`,
		})
	})
	if err != nil {
		t.Fatalf("upsert sync todo: %v", err)
	}

	row, err := noteDB.GetSyncTodoByID(ctx, "todo-1")
	if err != nil {
		t.Fatalf("get sync todo: %v", err)
	}
	if row == nil {
		t.Fatalf("expected sync todo row")
	}
	if row.SourceID != "hive-mind" {
		t.Fatalf("source_id=%q want=%q", row.SourceID, "hive-mind")
	}
	if row.TodoSection != "Next" {
		t.Fatalf("todo_section=%q want=%q", row.TodoSection, "Next")
	}
	if row.TaskText != "Call electrician" {
		t.Fatalf("task_text=%q want=%q", row.TaskText, "Call electrician")
	}
	if row.IsDone {
		t.Fatalf("is_done=%v want=false", row.IsDone)
	}
	if !row.SourcePath.Valid || row.SourcePath.String != sourcePath {
		t.Fatalf("source_path=%#v want=%q", row.SourcePath, sourcePath)
	}
	if !row.LineNumber.Valid || row.LineNumber.Int64 != int64(line) {
		t.Fatalf("line_number=%#v want=%d", row.LineNumber, line)
	}
}

func TestUpsertSyncTodo_RejectsInvalidTodoSection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		return tx.UpsertSyncTodo(SyncTodoUpsertParams{
			ID:          "todo-2",
			SourceID:    "hive-mind",
			TodoSection: "Backlog",
			TaskText:    "Invalid section task",
			Payload:     `{}`,
		})
	})
	if err == nil {
		t.Fatalf("expected invalid todo section error")
	}
}
