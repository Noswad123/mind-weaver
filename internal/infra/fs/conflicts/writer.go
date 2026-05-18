package conflicts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
)

type ArtifactWriter struct {
	baseDir string
	nowFn   func() time.Time
}

type artifactPayload struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	DeviceID      string    `json:"device_id"`

	EntityType syncops.EntityType `json:"entity_type"`
	EntityKey  string             `json:"entity_key"`

	Reason         string `json:"reason"`
	WinnerDeviceID string `json:"winner_device_id"`
	LoserDeviceID  string `json:"loser_device_id"`
	ServerCursor   string `json:"server_cursor"`

	LocalPayload  string `json:"local_payload"`
	RemotePayload string `json:"remote_payload"`
}

func NewArtifactWriter(baseDir string) *ArtifactWriter {
	return &ArtifactWriter{
		baseDir: baseDir,
		nowFn:   time.Now,
	}
}

func (w *ArtifactWriter) WriteJSONArtifact(deviceID string, event syncops.ConflictEvent) (string, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return "", errors.New("device id is required")
	}
	if strings.TrimSpace(event.EntityKey) == "" {
		return "", errors.New("entity key is required")
	}
	if strings.TrimSpace(event.Reason) == "" {
		return "", errors.New("conflict reason is required")
	}

	if err := os.MkdirAll(w.baseDir, 0o755); err != nil {
		return "", fmt.Errorf("create conflict artifact directory: %w", err)
	}

	ts := w.nowFn().UTC().Format("20060102T150405Z")
	entityKeySafe := sanitizeForFilename(event.EntityKey)
	filename := fmt.Sprintf("%s--%s--%s--%s.json", ts, sanitizeForFilename(deviceID), event.EntityType, entityKeySafe)
	path := filepath.Join(w.baseDir, filename)

	payload := artifactPayload{
		SchemaVersion:  "1",
		GeneratedAt:    w.nowFn().UTC(),
		DeviceID:       deviceID,
		EntityType:     event.EntityType,
		EntityKey:      event.EntityKey,
		Reason:         event.Reason,
		WinnerDeviceID: event.WinnerDeviceID,
		LoserDeviceID:  event.LoserDeviceID,
		ServerCursor:   event.ServerCursor,
		LocalPayload:   event.LocalPayload,
		RemotePayload:  event.RemotePayload,
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal conflict artifact payload: %w", err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("write conflict artifact: %w", err)
	}

	return path, nil
}

var unsafeFileChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func sanitizeForFilename(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, string(filepath.Separator), "_")
	v = strings.ReplaceAll(v, "/", "_")
	v = unsafeFileChars.ReplaceAllString(v, "_")
	v = strings.Trim(v, "._-")
	if v == "" {
		return "unknown"
	}
	return v
}
