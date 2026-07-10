package db

import (
	"context"
	"testing"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func TestGetSyncDiagnostics_EmptyDefaults(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	diag, err := noteDB.GetSyncDiagnostics(ctx, "last_server_cursor")
	if err != nil {
		t.Fatalf("get sync diagnostics: %v", err)
	}

	if diag.PendingOutboxCount != 0 {
		t.Fatalf("pending outbox=%d want=0", diag.PendingOutboxCount)
	}
	if diag.UnresolvedConflictCount != 0 {
		t.Fatalf("unresolved conflicts=%d want=0", diag.UnresolvedConflictCount)
	}
	if diag.LocalCursor != "0" {
		t.Fatalf("local cursor=%q want=0", diag.LocalCursor)
	}
}

func TestGetSyncDiagnostics_PopulatedValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	const cursorKey = "last_server_cursor"

	err := noteDB.WithTx(ctx, func(tx *Tx) error {
		if err := tx.EnqueueOutboxOperation(syncops.EntityNote, "notes/hub.md", syncops.OperationUpsert, `{"title":"A"}`); err != nil {
			return err
		}
		if err := tx.EnqueueOutboxOperation(syncops.EntityTodo, "todo-1", syncops.OperationUpsert, `{"id":"todo-1"}`); err != nil {
			return err
		}

		if err := tx.UpsertSyncTodo(SyncTodoUpsertParams{
			ID:          "todo-1",
			SourceID:    "hive-mind",
			TodoSection: "Inbox",
			TaskText:    "Investigate diagnostics",
			Payload:     `{"id":"todo-1"}`,
		}); err != nil {
			return err
		}

		if err := tx.SetSyncEntityVersion(syncops.EntityNote, "notes/hub.md", 3); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		t.Fatalf("seed diagnostics data: %v", err)
	}

	pending, err := noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	if len(pending) == 0 {
		t.Fatalf("expected pending outbox rows")
	}

	if err := noteDB.MarkOutboxAttemptFailed(ctx, pending[0].OpID, "network timeout"); err != nil {
		t.Fatalf("mark outbox attempt failed: %v", err)
	}
	if err := noteDB.MarkOutboxAcked(ctx, pending[1].OpID); err != nil {
		t.Fatalf("mark outbox acked: %v", err)
	}

	if err := noteDB.InsertSyncConflict(ctx, syncops.ConflictEvent{
		EntityType:    syncops.EntityNote,
		EntityKey:     "notes/hub.md",
		LocalPayload:  `{"content":"local"}`,
		RemotePayload: `{"content":"remote"}`,
		Reason:        "base_version_mismatch",
	}); err != nil {
		t.Fatalf("insert sync conflict: %v", err)
	}

	if err := noteDB.SetSyncState(ctx, cursorKey, "17"); err != nil {
		t.Fatalf("set sync state: %v", err)
	}

	diag, err := noteDB.GetSyncDiagnostics(ctx, cursorKey)
	if err != nil {
		t.Fatalf("get sync diagnostics: %v", err)
	}

	if diag.PendingOutboxCount != 1 {
		t.Fatalf("pending outbox=%d want=1", diag.PendingOutboxCount)
	}
	if diag.PendingOutboxRetriedCount != 1 {
		t.Fatalf("pending retried outbox=%d want=1", diag.PendingOutboxRetriedCount)
	}
	if diag.AckedOutboxCount != 1 {
		t.Fatalf("acked outbox=%d want=1", diag.AckedOutboxCount)
	}
	if diag.UnresolvedConflictCount != 1 {
		t.Fatalf("unresolved conflicts=%d want=1", diag.UnresolvedConflictCount)
	}
	if diag.TotalConflictCount != 1 {
		t.Fatalf("total conflicts=%d want=1", diag.TotalConflictCount)
	}
	if diag.SyncEntityVersionCount != 1 {
		t.Fatalf("sync entity version count=%d want=1", diag.SyncEntityVersionCount)
	}
	if diag.SyncedTodoCount != 1 {
		t.Fatalf("synced todo count=%d want=1", diag.SyncedTodoCount)
	}
	if diag.LocalCursor != "17" {
		t.Fatalf("local cursor=%q want=17", diag.LocalCursor)
	}
	if diag.PendingOutboxLatestFailure == "" {
		t.Fatalf("expected latest pending outbox failure to be populated")
	}
}
