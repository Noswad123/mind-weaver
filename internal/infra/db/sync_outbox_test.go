package db

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func schemaPathForTests(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("could not resolve caller path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	return filepath.Join(repoRoot, "db", "schema.sql")
}

func newTestNoteDB(t *testing.T) *NoteDb {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	noteDB, err := NewNoteDb(dbPath, schemaPathForTests(t))
	if err != nil {
		t.Fatalf("new test db: %v", err)
	}
	t.Cleanup(func() { _ = noteDB.Close() })
	return noteDB
}

func TestOutboxEnqueue_DedupesSamePendingPayload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		if err := tx.EnqueueOutboxOperation(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"A"}`); err != nil {
			return err
		}
		return tx.EnqueueOutboxOperation(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"A"}`)
	})
	if err != nil {
		t.Fatalf("enqueue duplicate pending payload: %v", err)
	}

	pending, err := noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending operation, got %d", len(pending))
	}

	err = noteDB.WithTx(ctx, func(tx *Tx) error {
		return tx.EnqueueOutboxOperation(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"B"}`)
	})
	if err != nil {
		t.Fatalf("enqueue changed payload: %v", err)
	}

	pending, err = noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending outbox (after changed payload): %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending operations after changed payload, got %d", len(pending))
	}
}

func TestOutboxEnqueue_DoesNotDedupeAcrossDifferentBaseVersion(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		if err := tx.EnqueueOutboxOperationWithBaseVersion(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"A"}`, 1); err != nil {
			return err
		}
		return tx.EnqueueOutboxOperationWithBaseVersion(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"A"}`, 2)
	})
	if err != nil {
		t.Fatalf("enqueue operations with different base versions: %v", err)
	}

	pending, err := noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending outbox: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending operations, got %d", len(pending))
	}
}

func TestOutboxMarkAcked_RemovesFromPending(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		return tx.EnqueueOutboxOperation(syncops.EntityNote, "notes/task.md", syncops.OperationDelete, `{"path":"notes/task.md"}`)
	})
	if err != nil {
		t.Fatalf("enqueue delete operation: %v", err)
	}

	pending, err := noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending operation, got %d", len(pending))
	}

	if err := noteDB.MarkOutboxAcked(ctx, pending[0].OpID); err != nil {
		t.Fatalf("mark outbox acked: %v", err)
	}

	pending, err = noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending after ack: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending operations after ack, got %d", len(pending))
	}
}

func TestSyncState_SetAndGet(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	const key = "last_server_cursor"
	if err := noteDB.SetSyncState(ctx, key, "101"); err != nil {
		t.Fatalf("set sync state (first): %v", err)
	}

	value, err := noteDB.GetSyncState(ctx, key)
	if err != nil {
		t.Fatalf("get sync state (first): %v", err)
	}
	if value == nil || *value != "101" {
		t.Fatalf("expected sync state=101, got %#v", value)
	}

	if err := noteDB.SetSyncState(ctx, key, "102"); err != nil {
		t.Fatalf("set sync state (update): %v", err)
	}

	value, err = noteDB.GetSyncState(ctx, key)
	if err != nil {
		t.Fatalf("get sync state (update): %v", err)
	}
	if value == nil || *value != "102" {
		t.Fatalf("expected sync state=102, got %#v", value)
	}
}

func TestSyncEntityVersion_ResolveSetAndIncrement(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	version, err := noteDB.GetSyncEntityVersion(ctx, syncops.EntityNote, "notes/hub.md")
	if err != nil {
		t.Fatalf("get initial sync entity version: %v", err)
	}
	if version != nil {
		t.Fatalf("expected nil initial version, got %#v", version)
	}

	err = noteDB.WithTx(ctx, func(tx *Tx) error {
		if err := tx.SetSyncEntityVersion(syncops.EntityNote, "notes/hub.md", 3); err != nil {
			return err
		}
		next, err := tx.IncrementSyncEntityVersion(syncops.EntityNote, "notes/hub.md")
		if err != nil {
			return err
		}
		if next != 4 {
			t.Fatalf("incremented version=%d want=4", next)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("set/increment sync entity version: %v", err)
	}

	version, err = noteDB.GetSyncEntityVersion(ctx, syncops.EntityNote, "notes/hub.md")
	if err != nil {
		t.Fatalf("get sync entity version: %v", err)
	}
	if version == nil || *version != 4 {
		t.Fatalf("expected version=4, got %#v", version)
	}
}
