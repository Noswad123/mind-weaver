package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type SyncTodoUpsertParams struct {
	ID          string
	SourceID    string
	SourcePath  *string
	TaskScope   *string
	TaskArea    *string
	TodoSection string
	TaskText    string
	IsDone      bool
	Meta        *string
	TaskOrder   int
	LineNumber  *int
	UpdatedAt   *string
	Payload     string
}

func normalizeNullableString(v *string) any {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func normalizeNullableInt(v *int) any {
	if v == nil {
		return nil
	}
	if *v < 0 {
		return nil
	}
	return *v
}

func normalizeTodoSection(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "Inbox", nil
	}
	switch strings.ToLower(v) {
	case "inbox":
		return "Inbox", nil
	case "next":
		return "Next", nil
	case "waiting":
		return "Waiting", nil
	default:
		return "", errors.New("todo_section must be Inbox, Next, or Waiting")
	}
}

func (t *Tx) UpsertSyncTodo(params SyncTodoUpsertParams) error {
	id := strings.TrimSpace(params.ID)
	if id == "" {
		return errors.New("sync todo id is required")
	}
	sourceID := strings.TrimSpace(params.SourceID)
	if sourceID == "" {
		return errors.New("sync todo source_id is required")
	}
	section, err := normalizeTodoSection(params.TodoSection)
	if err != nil {
		return err
	}
	taskText := strings.TrimSpace(params.TaskText)
	if taskText == "" {
		return errors.New("sync todo task_text is required")
	}

	payload := strings.TrimSpace(params.Payload)
	if payload == "" {
		payload = "{}"
	}

	updatedAt := normalizeNullableString(params.UpdatedAt)
	if updatedAt == nil {
		updatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err = t.tx.Exec(`
		INSERT INTO sync_todos (
			id,
			source_id,
			source_path,
			task_scope,
			task_area,
			todo_section,
			task_text,
			is_done,
			meta,
			task_order,
			line_number,
			updated_at,
			payload
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source_id = excluded.source_id,
			source_path = excluded.source_path,
			task_scope = excluded.task_scope,
			task_area = excluded.task_area,
			todo_section = excluded.todo_section,
			task_text = excluded.task_text,
			is_done = excluded.is_done,
			meta = excluded.meta,
			task_order = excluded.task_order,
			line_number = excluded.line_number,
			updated_at = excluded.updated_at,
			payload = excluded.payload
	`,
		id,
		sourceID,
		normalizeNullableString(params.SourcePath),
		normalizeNullableString(params.TaskScope),
		normalizeNullableString(params.TaskArea),
		section,
		taskText,
		params.IsDone,
		normalizeNullableString(params.Meta),
		params.TaskOrder,
		normalizeNullableInt(params.LineNumber),
		updatedAt,
		payload,
	)
	return err
}

func (t *Tx) DeleteSyncTodo(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("sync todo id is required")
	}

	_, err := t.tx.Exec(`DELETE FROM sync_todos WHERE id = ?`, id)
	return err
}

func (db *NoteDb) GetSyncTodoByID(ctx context.Context, id string) (*SyncTodoRow, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("sync todo id is required")
	}

	var row SyncTodoRow
	err := db.conn.QueryRowContext(ctx, `
		SELECT
			id,
			COALESCE(source_id, ''),
			source_path,
			task_scope,
			task_area,
			COALESCE(todo_section, 'Inbox'),
			COALESCE(task_text, ''),
			is_done,
			meta,
			COALESCE(task_order, 0),
			line_number,
			COALESCE(updated_at, ''),
			COALESCE(payload, '{}')
		FROM sync_todos
		WHERE id = ?
	`, id).Scan(
		&row.ID,
		&row.SourceID,
		&row.SourcePath,
		&row.TaskScope,
		&row.TaskArea,
		&row.TodoSection,
		&row.TaskText,
		&row.IsDone,
		&row.Meta,
		&row.TaskOrder,
		&row.LineNumber,
		&row.UpdatedAt,
		&row.Payload,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &row, nil
}
