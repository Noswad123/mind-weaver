package db

import (
	"context"
	sq "github.com/Noswad123/mind-weaver/internal/core/saved_query"
)


func (db *NoteDb) LoadSavedQueries(ctx context.Context) ([]sq.SavedQuery, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT name, sql FROM saved_queries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []sq.SavedQuery{}
	for rows.Next() {
		var q sq.SavedQuery
		if err := rows.Scan(&q.Name, &q.SQL); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}
