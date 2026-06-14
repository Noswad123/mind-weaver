package db

import (
	"context"
	"database/sql"
)

func (db *NoteDb) ListTaskGroupsForNote(noteID int) ([]TaskGroupRow, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, level, derived_group_id, status, raw_status, line_number
		FROM task_groups
		WHERE note_id = ?
		ORDER BY id ASC
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TaskGroupRow{}
	for rows.Next() {
		var r TaskGroupRow
		var derived sql.NullInt64

		if err := rows.Scan(&r.ID, &r.Name, &r.Level, &derived, &r.Status, &r.RawStatus, &r.LineNumber); err != nil {
			return nil, err
		}
		if derived.Valid {
			v := int(derived.Int64)
			r.DerivedGroupID = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) ListTodosForNote(noteID int) ([]TodoRow, error) {
	rows, err := db.conn.Query(`
		SELECT task_group_id, task, status, raw_status, depth, line_number
		FROM todos
		WHERE note_id = ?
		ORDER BY id ASC
	`, noteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TodoRow{}
	for rows.Next() {
		var r TodoRow
		if err := rows.Scan(&r.TaskGroupID, &r.Task, &r.Status, &r.RawStatus, &r.Depth, &r.LineNumber); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) ListTodoProjection(ctx context.Context) ([]TodoProjectionRow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT
			td.id,
			td.note_id,
			COALESCE(n.title, ''),
			n.path,
			tg.id,
			tg.name,
			td.task,
			td.status,
			td.raw_status,
			td.depth,
			COALESCE(td.line_number, 0)
		FROM todos td
		JOIN notes n ON n.id = td.note_id
		JOIN task_groups tg ON tg.id = td.task_group_id
		JOIN note_domains nd ON nd.note_id = n.id AND nd.domain = 'task-index'
		ORDER BY n.path ASC, COALESCE(tg.line_number, 0) ASC, COALESCE(td.line_number, 0) ASC, td.id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []TodoProjectionRow{}
	for rows.Next() {
		var r TodoProjectionRow
		if err := rows.Scan(
			&r.ID,
			&r.NoteID,
			&r.NoteTitle,
			&r.Path,
			&r.TaskGroupID,
			&r.TaskGroupName,
			&r.Task,
			&r.Status,
			&r.RawStatus,
			&r.Depth,
			&r.LineNumber,
		); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
