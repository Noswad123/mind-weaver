package db

import (
	"context"
	"testing"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func TestListSyncConflictsByFilter_UnresolvedOlderThan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	if err := noteDB.InsertSyncConflict(ctx, syncops.ConflictEvent{
		EntityType:    syncops.EntityNote,
		EntityKey:     "notes/old.md",
		LocalPayload:  `{"content":"local-old"}`,
		RemotePayload: `{"content":"remote-old"}`,
		Reason:        "base_version_mismatch",
	}); err != nil {
		t.Fatalf("insert old conflict: %v", err)
	}
	if err := noteDB.InsertSyncConflict(ctx, syncops.ConflictEvent{
		EntityType:    syncops.EntityTodo,
		EntityKey:     "todo-new",
		LocalPayload:  `{"id":"todo-new"}`,
		RemotePayload: `{"id":"todo-new"}`,
		Reason:        "base_version_mismatch",
	}); err != nil {
		t.Fatalf("insert new conflict: %v", err)
	}

	oldTime := time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := noteDB.conn.ExecContext(ctx, `UPDATE sync_conflicts SET created_at = ? WHERE entity_key = ?`, oldTime, "notes/old.md"); err != nil {
		t.Fatalf("backdate old conflict: %v", err)
	}

	cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
	rows, err := noteDB.ListSyncConflictsByFilter(ctx, SyncConflictFilter{
		Limit:          10,
		UnresolvedOnly: true,
		CreatedBefore:  &cutoff,
	})
	if err != nil {
		t.Fatalf("list conflicts by filter: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("filtered conflicts=%d want=1", len(rows))
	}
	if rows[0].EntityKey != "notes/old.md" {
		t.Fatalf("entity key=%q want=%q", rows[0].EntityKey, "notes/old.md")
	}
}

func TestMarkSyncConflictsResolved(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	if err := noteDB.InsertSyncConflict(ctx, syncops.ConflictEvent{
		EntityType:    syncops.EntityNote,
		EntityKey:     "notes/hub.md",
		LocalPayload:  `{"content":"local"}`,
		RemotePayload: `{"content":"remote"}`,
		Reason:        "base_version_mismatch",
	}); err != nil {
		t.Fatalf("insert conflict: %v", err)
	}

	rows, err := noteDB.ListSyncConflicts(ctx, 10)
	if err != nil {
		t.Fatalf("list conflicts: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("conflicts=%d want=1", len(rows))
	}

	if err := noteDB.MarkSyncConflictsResolved(ctx, []int64{rows[0].ID}, time.Now().UTC()); err != nil {
		t.Fatalf("mark conflicts resolved: %v", err)
	}

	rows, err = noteDB.ListSyncConflicts(ctx, 10)
	if err != nil {
		t.Fatalf("list conflicts after resolve: %v", err)
	}
	if !rows[0].ResolvedAt.Valid {
		t.Fatalf("expected resolved_at to be populated")
	}

	cutoff := time.Now().UTC().Add(24 * time.Hour)
	unresolved, err := noteDB.ListSyncConflictsByFilter(ctx, SyncConflictFilter{
		Limit:          10,
		UnresolvedOnly: true,
		CreatedBefore:  &cutoff,
	})
	if err != nil {
		t.Fatalf("list unresolved conflicts: %v", err)
	}
	if len(unresolved) != 0 {
		t.Fatalf("unresolved conflicts=%d want=0", len(unresolved))
	}
}
