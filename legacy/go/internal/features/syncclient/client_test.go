package syncclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
	"github.com/Noswad123/mind-weaver/internal/features/syncapi"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
	fsconflicts "github.com/Noswad123/mind-weaver/internal/infra/fs/conflicts"
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

func newTestNoteDB(t *testing.T) *db.NoteDb {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "notes.db")
	noteDB, err := db.NewNoteDb(dbPath, schemaPathForTests(t))
	if err != nil {
		t.Fatalf("new test db: %v", err)
	}
	t.Cleanup(func() { _ = noteDB.Close() })
	return noteDB
}

func newTestSyncClient(t *testing.T, noteDB *db.NoteDb, baseURL, deviceID, conflictsDir string) *Client {
	t.Helper()

	writer := fsconflicts.NewArtifactWriter(conflictsDir)
	client, err := New(noteDB, &http.Client{Timeout: 10 * time.Second}, Config{
		BaseURL:         baseURL,
		DeviceID:        deviceID,
		DeviceName:      deviceID,
		Platform:        "darwin",
		AppVersion:      "test",
		OutboxBatchSize: 100,
		PullLimit:       100,
	}, writer)
	if err != nil {
		t.Fatalf("new sync client: %v", err)
	}

	return client
}

func newTestSyncClientWithLimits(t *testing.T, noteDB *db.NoteDb, baseURL, deviceID, conflictsDir string, outboxLimit, pullLimit int) *Client {
	t.Helper()

	writer := fsconflicts.NewArtifactWriter(conflictsDir)
	client, err := New(noteDB, &http.Client{Timeout: 10 * time.Second}, Config{
		BaseURL:         baseURL,
		DeviceID:        deviceID,
		DeviceName:      deviceID,
		Platform:        "darwin",
		AppVersion:      "test",
		OutboxBatchSize: outboxLimit,
		PullLimit:       pullLimit,
	}, writer)
	if err != nil {
		t.Fatalf("new sync client with limits: %v", err)
	}

	return client
}

func enqueueNoteUpsert(t *testing.T, noteDB *db.NoteDb, path, title, content string) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"path":    path,
		"title":   title,
		"content": content,
		"tags":    []string{"sync"},
		"domains": []string{"project"},
	})
	if err != nil {
		t.Fatalf("marshal note payload: %v", err)
	}

	err = noteDB.WithTx(context.Background(), func(tx *db.Tx) error {
		return tx.EnqueueOutboxOperation(syncops.EntityNote, path, syncops.OperationUpsert, string(payload))
	})
	if err != nil {
		t.Fatalf("enqueue outbox note upsert: %v", err)
	}
}

func enqueueTodoUpsert(t *testing.T, noteDB *db.NoteDb, id, sourceID, section, text string, done bool) {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"id":           id,
		"source_id":    sourceID,
		"source_path":  "projects/hive/hub.md",
		"task_scope":   "project",
		"task_area":    "Action",
		"domains":      []string{"task-index"},
		"task_active":  true,
		"todo_section": section,
		"text":         text,
		"done":         done,
		"meta":         "p2 e:m",
		"order":        1,
		"line":         18,
	})
	if err != nil {
		t.Fatalf("marshal todo payload: %v", err)
	}

	err = noteDB.WithTx(context.Background(), func(tx *db.Tx) error {
		return tx.EnqueueOutboxOperation(syncops.EntityTodo, id, syncops.OperationUpsert, string(payload))
	})
	if err != nil {
		t.Fatalf("enqueue outbox todo upsert: %v", err)
	}
}

func loadNoteByPath(ctx context.Context, noteDB *db.NoteDb, path string) (*db.NoteRow, error) {
	var noteID *int
	err := noteDB.WithTx(ctx, func(tx *db.Tx) error {
		var err error
		noteID, err = tx.ResolveNoteIDByPath(path)
		return err
	})
	if err != nil {
		return nil, err
	}
	if noteID == nil {
		return nil, nil
	}

	row, err := noteDB.GetNoteByID(ctx, *noteID)
	if err != nil {
		return nil, err
	}

	return &row, nil
}

func TestRunSyncOnce_PushesThenPullsAcrossDevices(t *testing.T) {
	ctx := context.Background()

	server := syncapi.NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	deviceA := newTestNoteDB(t)
	deviceB := newTestNoteDB(t)

	clientA := newTestSyncClient(t, deviceA, httpServer.URL, "device-a", t.TempDir())
	clientB := newTestSyncClient(t, deviceB, httpServer.URL, "device-b", t.TempDir())

	enqueueNoteUpsert(t, deviceA, "notes/hub.md", "Hub", "content from device A")

	if _, err := clientA.RunSyncOnce(ctx); err != nil {
		t.Fatalf("sync device A: %v", err)
	}

	if _, err := clientB.RunSyncOnce(ctx); err != nil {
		t.Fatalf("sync device B: %v", err)
	}

	noteB, err := loadNoteByPath(ctx, deviceB, "notes/hub.md")
	if err != nil {
		t.Fatalf("load note on device B: %v", err)
	}
	if noteB == nil {
		t.Fatalf("expected note on device B after pull")
	}
	if noteB.Content != "content from device A" {
		t.Fatalf("device B note content=%q want=%q", noteB.Content, "content from device A")
	}

	cursor, err := deviceB.GetSyncState(ctx, SyncStateLastServerCursorKey)
	if err != nil {
		t.Fatalf("read device B cursor state: %v", err)
	}
	if cursor == nil || *cursor == "" || *cursor == "0" {
		t.Fatalf("expected non-zero cursor persisted on device B, got %#v", cursor)
	}
}

func TestRunSyncOnce_ConflictWritesArtifactAndConverges(t *testing.T) {
	ctx := context.Background()

	server := syncapi.NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	deviceA := newTestNoteDB(t)
	deviceB := newTestNoteDB(t)
	conflictsDirB := t.TempDir()

	clientA := newTestSyncClient(t, deviceA, httpServer.URL, "device-a", t.TempDir())
	clientB := newTestSyncClient(t, deviceB, httpServer.URL, "device-b", conflictsDirB)

	enqueueNoteUpsert(t, deviceA, "notes/hub.md", "Hub", "alpha")
	if _, err := clientA.RunSyncOnce(ctx); err != nil {
		t.Fatalf("initial sync device A: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	enqueueNoteUpsert(t, deviceB, "notes/hub.md", "Hub", "bravo")
	resultB, err := clientB.RunSyncOnce(ctx)
	if err != nil {
		t.Fatalf("sync device B with conflict: %v", err)
	}
	if resultB.ConflictsLogged == 0 {
		t.Fatalf("expected at least one logged conflict, got %d", resultB.ConflictsLogged)
	}
	if resultB.ConflictArtifactsWrote == 0 {
		t.Fatalf("expected at least one conflict artifact write, got %d", resultB.ConflictArtifactsWrote)
	}

	entries, err := os.ReadDir(conflictsDirB)
	if err != nil {
		t.Fatalf("read conflicts dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected conflict artifact file in %s", conflictsDirB)
	}

	conflicts, err := deviceB.ListSyncConflicts(ctx, 10)
	if err != nil {
		t.Fatalf("list local sync conflicts: %v", err)
	}
	if len(conflicts) == 0 {
		t.Fatalf("expected local sync conflict rows")
	}

	if _, err := clientA.RunSyncOnce(ctx); err != nil {
		t.Fatalf("final sync device A: %v", err)
	}

	noteA, err := loadNoteByPath(ctx, deviceA, "notes/hub.md")
	if err != nil {
		t.Fatalf("load note on device A: %v", err)
	}
	noteB, err := loadNoteByPath(ctx, deviceB, "notes/hub.md")
	if err != nil {
		t.Fatalf("load note on device B: %v", err)
	}
	if noteA == nil || noteB == nil {
		t.Fatalf("expected notes on both devices after convergence, got A=%v B=%v", noteA != nil, noteB != nil)
	}
	if noteA.Content != noteB.Content {
		t.Fatalf("expected converged note content, got A=%q B=%q", noteA.Content, noteB.Content)
	}
}

func TestRunSyncOnce_UsesStoredEntityBaseVersionInPush(t *testing.T) {
	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	const entityPath = "notes/hub.md"
	err := noteDB.WithTx(ctx, func(tx *db.Tx) error {
		return tx.SetSyncEntityVersion(syncops.EntityNote, entityPath, 7)
	})
	if err != nil {
		t.Fatalf("seed sync entity version: %v", err)
	}

	enqueueNoteUpsert(t, noteDB, entityPath, "Hub", "base-version test")

	capturedBaseVersion := -1
	var pushDecodeErr error
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sync/push", func(w http.ResponseWriter, r *http.Request) {
		var req pushRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			pushDecodeErr = err
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid push payload"})
			return
		}
		if len(req.Operations) != 1 {
			pushDecodeErr = fmt.Errorf("operations len=%d want=1", len(req.Operations))
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid operation count"})
			return
		}
		capturedBaseVersion = req.Operations[0].BaseVersion

		_ = json.NewEncoder(w).Encode(pushResponse{
			Accepted:     []string{req.Operations[0].OpID},
			Rejected:     []pushReject{},
			ServerCursor: "1",
			Conflicts:    []pushConflict{},
		})
	})
	mux.HandleFunc("/v1/sync/pull", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(pullResponse{Operations: []pullOperation{}, NextCursor: "1", HasMore: false})
	})
	mux.HandleFunc("/v1/sync/state", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"server_time": "2026-04-05T00:00:00Z", "latest_cursor": "1"})
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := newTestSyncClient(t, noteDB, httpServer.URL, "device-a", t.TempDir())
	if _, err := client.RunSyncOnce(ctx); err != nil {
		t.Fatalf("run sync once: %v", err)
	}
	if pushDecodeErr != nil {
		t.Fatalf("push handler validation failed: %v", pushDecodeErr)
	}

	if capturedBaseVersion != 7 {
		t.Fatalf("captured base_version=%d want=7", capturedBaseVersion)
	}
}

func TestRunSyncOnce_AppliesTodoEntityPullOperations(t *testing.T) {
	ctx := context.Background()

	server := syncapi.NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	deviceA := newTestNoteDB(t)
	deviceB := newTestNoteDB(t)

	clientA := newTestSyncClient(t, deviceA, httpServer.URL, "device-a", t.TempDir())
	clientB := newTestSyncClient(t, deviceB, httpServer.URL, "device-b", t.TempDir())

	enqueueTodoUpsert(t, deviceA, "todo-1", "hive-mind", "Next", "Call electrician", false)

	if _, err := clientA.RunSyncOnce(ctx); err != nil {
		t.Fatalf("sync device A: %v", err)
	}

	if _, err := clientB.RunSyncOnce(ctx); err != nil {
		t.Fatalf("sync device B: %v", err)
	}

	todoRow, err := deviceB.GetSyncTodoByID(ctx, "todo-1")
	if err != nil {
		t.Fatalf("load sync todo on device B: %v", err)
	}
	if todoRow == nil {
		t.Fatalf("expected synced todo row on device B")
	}
	if todoRow.TaskText != "Call electrician" {
		t.Fatalf("todo task_text=%q want=%q", todoRow.TaskText, "Call electrician")
	}
	if todoRow.TodoSection != "Next" {
		t.Fatalf("todo section=%q want=%q", todoRow.TodoSection, "Next")
	}
	if todoRow.SourceID != "hive-mind" {
		t.Fatalf("todo source_id=%q want=%q", todoRow.SourceID, "hive-mind")
	}
	if todoRow.IsDone {
		t.Fatalf("todo is_done=%v want=false", todoRow.IsDone)
	}

	version, err := deviceB.GetSyncEntityVersion(ctx, syncops.EntityTodo, "todo-1")
	if err != nil {
		t.Fatalf("read sync todo entity version: %v", err)
	}
	if version == nil || *version == 0 {
		t.Fatalf("expected sync todo entity version > 0, got %#v", version)
	}
}

func TestRunSyncWithRetry_RetriesTransientPushFailure(t *testing.T) {
	ctx := context.Background()
	noteDB := newTestNoteDB(t)
	enqueueNoteUpsert(t, noteDB, "notes/retry.md", "Retry", "payload")

	pushCalls := 0
	var pushDecodeErr error
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sync/push", func(w http.ResponseWriter, r *http.Request) {
		pushCalls++
		if pushCalls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "transient upstream failure"})
			return
		}

		var req pushRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			pushDecodeErr = err
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid push payload"})
			return
		}

		accepted := make([]string, 0, len(req.Operations))
		for _, op := range req.Operations {
			accepted = append(accepted, op.OpID)
		}

		_ = json.NewEncoder(w).Encode(pushResponse{
			Accepted:     accepted,
			Rejected:     []pushReject{},
			ServerCursor: "1",
			Conflicts:    []pushConflict{},
		})
	})
	mux.HandleFunc("/v1/sync/pull", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(pullResponse{Operations: []pullOperation{}, NextCursor: "1", HasMore: false})
	})
	mux.HandleFunc("/v1/sync/state", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"server_time": "2026-04-05T00:00:00Z", "latest_cursor": "1"})
	})

	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	client := newTestSyncClient(t, noteDB, httpServer.URL, "device-a", t.TempDir())

	result, err := client.RunSyncWithRetry(ctx, RetryConfig{
		MaxAttempts: 2,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("run sync with retry: %v", err)
	}
	if pushDecodeErr != nil {
		t.Fatalf("push handler validation failed: %v", pushDecodeErr)
	}
	if pushCalls != 2 {
		t.Fatalf("push call count=%d want=2", pushCalls)
	}
	if result.PushedAccepted != 1 {
		t.Fatalf("accepted=%d want=1", result.PushedAccepted)
	}

	pending, err := noteDB.ListPendingOutbox(ctx, 10)
	if err != nil {
		t.Fatalf("list pending outbox after retry sync: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected outbox empty after successful retry sync, pending=%d", len(pending))
	}

	if result.CursorLag != 0 {
		t.Fatalf("cursor lag=%d want=0", result.CursorLag)
	}
	if result.ServerLatestCursor != "1" {
		t.Fatalf("server latest cursor=%q want=1", result.ServerLatestCursor)
	}
	if result.ConflictRate() != 0 {
		t.Fatalf("conflict rate=%f want=0", result.ConflictRate())
	}

	if result.FinalCursor == "" {
		t.Fatalf("final cursor should be populated")
	}

}

func TestRunSyncWorker_MultiDeviceOfflineSoakConverges(t *testing.T) {
	ctx := context.Background()

	server := syncapi.NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	deviceA := newTestNoteDB(t)
	deviceB := newTestNoteDB(t)
	deviceC := newTestNoteDB(t)

	clientA := newTestSyncClientWithLimits(t, deviceA, httpServer.URL, "device-a", t.TempDir(), 2, 2)
	clientB := newTestSyncClientWithLimits(t, deviceB, httpServer.URL, "device-b", t.TempDir(), 2, 2)
	clientC := newTestSyncClientWithLimits(t, deviceC, httpServer.URL, "device-c", t.TempDir(), 2, 2)

	type node struct {
		id     string
		db     *db.NoteDb
		client *Client
	}
	nodes := []node{
		{id: "device-a", db: deviceA, client: clientA},
		{id: "device-b", db: deviceB, client: clientB},
		{id: "device-c", db: deviceC, client: clientC},
	}

	expected := map[string]string{}

	for i := 0; i < 15; i++ {
		n := nodes[i%len(nodes)]
		path := fmt.Sprintf("notes/soak/%s-%02d.md", n.id, i)
		content := fmt.Sprintf("soak content from %s cycle %d", n.id, i)
		enqueueNoteUpsert(t, n.db, path, "soak", content)
		expected[path] = content

		if n.id == "device-b" && i < 6 {
			continue
		}

		if _, err := n.client.RunSyncOnce(ctx); err != nil {
			t.Fatalf("sync %s during soak cycle %d: %v", n.id, i, err)
		}

		if i%2 == 0 {
			for _, other := range nodes {
				if other.id == n.id {
					continue
				}
				if _, err := other.client.RunSyncOnce(ctx); err != nil {
					t.Fatalf("sync %s during fanout cycle %d: %v", other.id, i, err)
				}
			}
		}
	}

	originalBaseURL := clientB.cfg.BaseURL
	clientB.cfg.BaseURL = "http://127.0.0.1:1"
	enqueueNoteUpsert(t, deviceB, "notes/soak/device-b-offline.md", "offline", "offline pending change")
	if _, err := clientB.RunSyncWithRetry(ctx, RetryConfig{MaxAttempts: 1, BaseDelay: 1 * time.Millisecond, MaxDelay: 5 * time.Millisecond}); err == nil {
		t.Fatalf("expected offline sync failure for device-b")
	}
	pendingWhileOffline, err := deviceB.ListPendingOutbox(ctx, 20)
	if err != nil {
		t.Fatalf("list pending while offline: %v", err)
	}
	if len(pendingWhileOffline) == 0 {
		t.Fatalf("expected pending outbox entries while offline")
	}
	clientB.cfg.BaseURL = originalBaseURL
	expected["notes/soak/device-b-offline.md"] = "offline pending change"

	for round := 0; round < 8; round++ {
		for _, n := range nodes {
			result, err := n.client.RunSyncOnce(ctx)
			if err != nil {
				t.Fatalf("final convergence sync %s round %d: %v", n.id, round, err)
			}
			if round == 7 && result.CursorLag != 0 {
				t.Fatalf("expected zero cursor lag for %s, got %d", n.id, result.CursorLag)
			}
		}
	}

	for path, want := range expected {
		for _, n := range nodes {
			row, err := loadNoteByPath(ctx, n.db, path)
			if err != nil {
				t.Fatalf("load %s on %s: %v", path, n.id, err)
			}
			if row == nil {
				t.Fatalf("expected %s on %s after soak convergence", path, n.id)
			}
			if row.Content != want {
				t.Fatalf("content mismatch for %s on %s got=%q want=%q", path, n.id, row.Content, want)
			}
		}
	}

	for _, n := range nodes {
		pending, err := n.db.ListPendingOutbox(ctx, 20)
		if err != nil {
			t.Fatalf("list pending outbox for %s: %v", n.id, err)
		}
		if len(pending) != 0 {
			t.Fatalf("expected empty pending outbox for %s, got %d", n.id, len(pending))
		}
	}
}
