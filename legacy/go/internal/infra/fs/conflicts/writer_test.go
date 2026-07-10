package conflicts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

func TestWriteJSONArtifact_UsesTimestampDeviceAndSanitizedEntityKey(t *testing.T) {
	t.Parallel()

	baseDir := filepath.Join(t.TempDir(), "conflicts")
	w := NewArtifactWriter(baseDir)
	fixed := time.Date(2026, time.April, 5, 1, 2, 3, 0, time.UTC)
	w.nowFn = func() time.Time { return fixed }

	event := syncops.ConflictEvent{
		EntityType:     syncops.EntityNote,
		EntityKey:      "notes/Projects/Hive Mind/hub.md",
		Reason:         "irresolvable",
		WinnerDeviceID: "work-mac",
		LoserDeviceID:  "personal-mac",
		ServerCursor:   "42",
		LocalPayload:   `{"title":"local"}`,
		RemotePayload:  `{"title":"remote"}`,
	}

	artifactPath, err := w.WriteJSONArtifact("iphone 15 pro", event)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	filename := filepath.Base(artifactPath)
	if wantPrefix := "20260405T010203Z--iphone_15_pro--note--"; !strings.HasPrefix(filename, wantPrefix) {
		t.Fatalf("artifact filename %q does not start with %q", filename, wantPrefix)
	}
	if !strings.HasSuffix(filename, ".json") {
		t.Fatalf("artifact filename %q should end with .json", filename)
	}
	if strings.Contains(filename, " ") || strings.Contains(filename, "/") {
		t.Fatalf("artifact filename %q contains unsafe separators", filename)
	}
}

func TestWriteJSONArtifact_PersistsExpectedPayloadFields(t *testing.T) {
	t.Parallel()

	baseDir := filepath.Join(t.TempDir(), "conflicts")
	w := NewArtifactWriter(baseDir)
	fixed := time.Date(2026, time.April, 5, 1, 2, 3, 0, time.UTC)
	w.nowFn = func() time.Time { return fixed }

	event := syncops.ConflictEvent{
		EntityType:     syncops.EntityTodo,
		EntityKey:      "todo-123",
		Reason:         "irresolvable",
		WinnerDeviceID: "phone",
		LoserDeviceID:  "laptop",
		ServerCursor:   "9001",
		LocalPayload:   `{"status":"doing"}`,
		RemotePayload:  `{"status":"done"}`,
	}

	artifactPath, err := w.WriteJSONArtifact("desktop", event)
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}

	b, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	var payload artifactPayload
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatalf("unmarshal artifact: %v", err)
	}

	if payload.SchemaVersion != "1" {
		t.Fatalf("schema_version=%q want 1", payload.SchemaVersion)
	}
	if payload.DeviceID != "desktop" {
		t.Fatalf("device_id=%q want desktop", payload.DeviceID)
	}
	if payload.WinnerDeviceID != "phone" || payload.LoserDeviceID != "laptop" {
		t.Fatalf("winner/loser mismatch: winner=%q loser=%q", payload.WinnerDeviceID, payload.LoserDeviceID)
	}
	if payload.ServerCursor != "9001" {
		t.Fatalf("server_cursor=%q want 9001", payload.ServerCursor)
	}
	if payload.LocalPayload != `{"status":"doing"}` || payload.RemotePayload != `{"status":"done"}` {
		t.Fatalf("payload snapshots mismatch: local=%q remote=%q", payload.LocalPayload, payload.RemotePayload)
	}
}
