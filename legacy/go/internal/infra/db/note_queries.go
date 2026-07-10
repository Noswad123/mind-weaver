package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
)

func (db *NoteDb) List(ctx context.Context, limit, offset int) ([]NoteRow, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, path, COALESCE(title,''), COALESCE(updated_at,'')
		FROM notes
		ORDER BY updated_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Path, &r.Title, &r.UpdatedAt); err != nil {
			return nil, err
		}
		r.Tags, _ = db.getTagsForNote(r.ID)
		r.Domains, _ = db.getDomainsForNote(r.ID)
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) GetNoteByID(ctx context.Context, id int) (NoteRow, error) {
	row := db.conn.QueryRowContext(ctx, `SELECT id, COALESCE(title,''), path, COALESCE(content,'') FROM notes WHERE id = ?`, id)

	var r NoteRow
	if err := row.Scan(&r.ID, &r.Title, &r.Path, &r.Content); err != nil {
		return NoteRow{}, err
	}

	tags, _ := db.getTagsForNote(r.ID)
	domains, _ := db.getDomainsForNote(r.ID)
	links, _ := db.getLinksForNote(r.ID)
	r.Tags = tags
	r.Domains = domains
	r.Links = links

	return r, nil
}

func (db *NoteDb) ListNotesByPaths(ctx context.Context, paths []string) (map[string]NoteRow, error) {
	out := map[string]NoteRow{}
	if len(paths) == 0 {
		return out, nil
	}

	seen := map[string]struct{}{}
	unique := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		unique = append(unique, path)
	}
	if len(unique) == 0 {
		return out, nil
	}

	placeholders := strings.Repeat("?,", len(unique))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(unique))
	for i, path := range unique {
		args[i] = path
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, path, COALESCE(title,''), COALESCE(updated_at,'')
		FROM notes
		WHERE path IN (`+placeholders+`)
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Path, &r.Title, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out[r.Path] = r
	}
	return out, rows.Err()
}

func (db *NoteDb) SearchNotesByName(ctx context.Context, input string) ([]NoteRow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT DISTINCT id, COALESCE(title,''), path, COALESCE(content,'')
		FROM notes
		WHERE LOWER(title) LIKE '%' || LOWER(?) || '%'
	`, input)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Content); err != nil {
			return nil, err
		}
		r.Tags, _ = db.getTagsForNote(r.ID)
		r.Domains, _ = db.getDomainsForNote(r.ID)
		r.Links, _ = db.getLinksForNote(r.ID)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (db *NoteDb) ListByTags(ctx context.Context, tags []string) ([]NoteRow, error) {
	if len(tags) == 0 {
		return []NoteRow{}, nil
	}

	placeholders := strings.Repeat("?,", len(tags))
	placeholders = placeholders[:len(placeholders)-1]

	query := `
		SELECT DISTINCT n.id, COALESCE(n.title,''), n.path, COALESCE(n.content,'')
		FROM notes n
		JOIN tags t ON n.id = t.note_id
		WHERE t.tag IN (` + placeholders + `)`

	args := make([]any, len(tags))
	for i, tag := range tags {
		args[i] = tag
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Content); err != nil {
			return nil, err
		}
		r.Tags, _ = db.getTagsForNote(r.ID)
		r.Domains, _ = db.getDomainsForNote(r.ID)
		r.Links, _ = db.getLinksForNote(r.ID)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (db *NoteDb) ListByDomain(ctx context.Context, domain string) ([]NoteRow, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return []NoteRow{}, nil
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT DISTINCT n.id, COALESCE(n.title,''), n.path, COALESCE(n.content,'')
		FROM notes n
		JOIN note_domains d ON n.id = d.note_id
		WHERE d.domain = ?
		ORDER BY n.updated_at DESC, n.id DESC
	`, domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Title, &r.Path, &r.Content); err != nil {
			return nil, err
		}
		r.Tags, _ = db.getTagsForNote(r.ID)
		r.Domains, _ = db.getDomainsForNote(r.ID)
		r.Links, _ = db.getLinksForNote(r.ID)
		results = append(results, r)
	}
	return results, rows.Err()
}

func (db *NoteDb) ListDomains(ctx context.Context) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT DISTINCT domain
		FROM note_domains
		WHERE TRIM(domain) != ''
		ORDER BY domain
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	domains := []string{}
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			return nil, err
		}
		domains = append(domains, domain)
	}
	return domains, rows.Err()
}

func (db *NoteDb) GetNoteIdByUid(ctx context.Context, uid string) (*int, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return nil, nil
	}

	var noteID int
	err := db.conn.QueryRowContext(ctx, `SELECT note_id FROM note_ids WHERE note_uid = ?`, uid).Scan(&noteID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &noteID, nil
}

func (db *NoteDb) GetAllNotePaths(ctx context.Context) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `SELECT path FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func (db *NoteDb) DeleteNoteByPath(ctx context.Context, path string) error {
	_, err := db.conn.ExecContext(ctx, `DELETE FROM notes WHERE path = ?`, path)
	return err
}

func (db *NoteDb) Load() ([]NoteRow, error) {
	rows, err := db.conn.Query(`SELECT title, path, content FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NoteRow
	for rows.Next() {
		var title, path, content string
		if err := rows.Scan(&title, &path, &content); err != nil {
			continue
		}
		results = append(results, NoteRow{
			Title:   title,
			Path:    path,
			Content: content,
		})
	}

	return results, rows.Err()
}

func (db *NoteDb) ListWorkspaceNotePaths(ctx context.Context) ([]string, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT path FROM notes
		WHERE path LIKE '%/hub.md'
		   OR path = 'hub.md'
	`)
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
		if strings.EqualFold(path, notefiles.HubNoteFilename) { // root folder hub excluded
			continue
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func (db *NoteDb) ListNoteMetas() ([]NoteMeta, error) {
	rows, err := db.conn.Query(`SELECT id, path FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []NoteMeta
	for rows.Next() {
		var meta NoteMeta
		if err := rows.Scan(&meta.ID, &meta.Path); err != nil {
			continue
		}
		results = append(results, meta)
	}

	return results, rows.Err()
}

func (db *NoteDb) UpdateNoteContent(noteID int, newPath string, newTitle string, newContent string) error {
	timestamp := time.Now().Format(time.RFC3339)

	res, err := db.conn.Exec(`UPDATE notes SET path = ?, title = ?, content = ?, updated_at = ? WHERE id = ?`,
		newPath, newTitle, newContent, timestamp, noteID)
	if err != nil {
		return fmt.Errorf("failed to update note record for ID %d: %w", noteID, err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected for note ID %d: %w", noteID, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no note found with ID %d to update", noteID)
	}
	return nil
}

func (db *NoteDb) ListNotesForDomainValidation() ([]NoteRow, error) {
	rows, err := db.conn.Query(`SELECT id, path, content FROM notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []NoteRow{}
	for rows.Next() {
		var r NoteRow
		if err := rows.Scan(&r.ID, &r.Path, &r.Content); err != nil {
			return nil, err
		}

		out = append(out, r)
	}
	return out, rows.Err()
}
