package syncclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/syncops"
	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

const (
	SyncStateLastServerCursorKey = "last_server_cursor"
	defaultBatchSize             = 100
	defaultPullLimit             = 100
)

type Config struct {
	BaseURL         string
	AuthToken       string
	DeviceID        string
	DeviceName      string
	Platform        string
	AppVersion      string
	OutboxBatchSize int
	PullLimit       int
}

type ConflictArtifactWriter interface {
	WriteJSONArtifact(deviceID string, event syncops.ConflictEvent) (string, error)
}

type Client struct {
	noteDB         *db.NoteDb
	httpClient     *http.Client
	cfg            Config
	artifactWriter ConflictArtifactWriter
	nowFn          func() time.Time
}

type SyncResult struct {
	PushedAccepted         int
	PushedRejected         int
	ConflictsLogged        int
	ConflictArtifactsWrote int
	PulledApplied          int
	FinalCursor            string
	ServerLatestCursor     string
	CursorLag              int64
}

func (r SyncResult) ConflictRate() float64 {
	if r.PushedAccepted <= 0 {
		return 0
	}
	return float64(r.ConflictsLogged) / float64(r.PushedAccepted)
}

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	JitterRatio float64
}

type WorkerConfig struct {
	Iterations int
	Interval   time.Duration
	Retry      RetryConfig
}

type WorkerResult struct {
	IterationsRequested int
	IterationsCompleted int
	Aggregate           SyncResult
	Last                SyncResult
}

type httpStatusError struct {
	Operation  string
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	if e == nil {
		return "http status error"
	}
	return fmt.Sprintf("%s request failed: status %d body=%s", e.Operation, e.StatusCode, strings.TrimSpace(e.Body))
}

type pushOperation struct {
	OpID           string          `json:"op_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	EntityType     string          `json:"entity_type"`
	EntityID       string          `json:"entity_id"`
	OpType         string          `json:"op_type"`
	Payload        json.RawMessage `json:"payload"`
	ClientUpdated  string          `json:"client_updated_at"`
	BaseVersion    int             `json:"base_version"`
}

type pushRequest struct {
	DeviceID   string          `json:"device_id"`
	DeviceName string          `json:"device_name,omitempty"`
	Platform   string          `json:"platform,omitempty"`
	AppVersion string          `json:"app_version,omitempty"`
	Operations []pushOperation `json:"operations"`
}

type pushReject struct {
	OpID   string `json:"op_id"`
	Reason string `json:"reason"`
}

type pushConflict struct {
	EntityType     string `json:"entity_type"`
	EntityID       string `json:"entity_id"`
	Winner         string `json:"winner"`
	Loser          string `json:"loser"`
	Reason         string `json:"reason"`
	WinnerDeviceID string `json:"winner_device_id"`
	LoserDeviceID  string `json:"loser_device_id"`
}

type pushResponse struct {
	Accepted     []string       `json:"accepted"`
	Rejected     []pushReject   `json:"rejected"`
	ServerCursor string         `json:"server_cursor"`
	Conflicts    []pushConflict `json:"conflicts"`
}

type pullOperation struct {
	OpID           string          `json:"op_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	EntityType     string          `json:"entity_type"`
	EntityID       string          `json:"entity_id"`
	OpType         string          `json:"op_type"`
	Payload        json.RawMessage `json:"payload"`
	ClientUpdated  string          `json:"client_updated_at"`
	BaseVersion    int             `json:"base_version"`
}

type pullResponse struct {
	Operations []pullOperation `json:"operations"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

type notePayload struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
	Domains []string `json:"domains"`
}

type todoPayload struct {
	ID          string   `json:"id"`
	SourceID    string   `json:"source_id"`
	SourcePath  string   `json:"source_path"`
	TaskScope   string   `json:"task_scope"`
	TaskArea    string   `json:"task_area"`
	Domains     []string `json:"domains,omitempty"`
	TaskActive  *bool    `json:"task_active,omitempty"`
	TodoSection string   `json:"todo_section"`
	Text        string   `json:"text"`
	Done        bool     `json:"done"`
	Meta        string   `json:"meta,omitempty"`
	Order       int      `json:"order,omitempty"`
	Line        int      `json:"line,omitempty"`
	UpdatedAt   *string  `json:"updated_at,omitempty"`

	// Backward-compatible aliases (legacy shape)
	LegacyTitle  string `json:"title,omitempty"`
	LegacyStatus string `json:"status,omitempty"`
}

func New(noteDB *db.NoteDb, httpClient *http.Client, cfg Config, artifactWriter ConflictArtifactWriter) (*Client, error) {
	if noteDB == nil {
		return nil, fmt.Errorf("note db is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("base url is required")
	}

	deviceID := strings.TrimSpace(cfg.DeviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("device id is required")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	if cfg.OutboxBatchSize <= 0 {
		cfg.OutboxBatchSize = defaultBatchSize
	}
	if cfg.PullLimit <= 0 {
		cfg.PullLimit = defaultPullLimit
	}

	cfg.BaseURL = strings.TrimRight(baseURL, "/")
	cfg.DeviceID = deviceID
	cfg.DeviceName = strings.TrimSpace(cfg.DeviceName)
	cfg.Platform = strings.TrimSpace(cfg.Platform)
	cfg.AppVersion = strings.TrimSpace(cfg.AppVersion)
	cfg.AuthToken = strings.TrimSpace(cfg.AuthToken)

	return &Client{
		noteDB:         noteDB,
		httpClient:     httpClient,
		cfg:            cfg,
		artifactWriter: artifactWriter,
		nowFn:          time.Now,
	}, nil
}

func (c *Client) RunSyncOnce(ctx context.Context) (SyncResult, error) {
	result := SyncResult{FinalCursor: "0"}

	cursor, err := c.readCursor(ctx)
	if err != nil {
		return result, fmt.Errorf("read local sync cursor: %w", err)
	}
	result.FinalCursor = cursor

	pending, err := c.noteDB.ListPendingOutbox(ctx, c.cfg.OutboxBatchSize)
	if err != nil {
		return result, fmt.Errorf("list pending outbox: %w", err)
	}

	if len(pending) > 0 {
		ops := buildPushOperations(pending)
		pushResp, err := c.push(ctx, pushRequest{
			DeviceID:   c.cfg.DeviceID,
			DeviceName: c.cfg.DeviceName,
			Platform:   c.cfg.Platform,
			AppVersion: c.cfg.AppVersion,
			Operations: ops,
		})
		if err != nil {
			for _, row := range pending {
				_ = c.noteDB.MarkOutboxAttemptFailed(ctx, row.OpID, err.Error())
			}
			return result, err
		}

		pendingByOpID := map[string]db.SyncOutboxRow{}
		for _, row := range pending {
			pendingByOpID[row.OpID] = row
		}

		rejectedByOpID := map[string]pushReject{}
		for _, reject := range pushResp.Rejected {
			rejectedByOpID[reject.OpID] = reject
			if _, ok := pendingByOpID[reject.OpID]; !ok {
				continue
			}
			if err := c.noteDB.MarkOutboxAttemptFailed(ctx, reject.OpID, reject.Reason); err != nil {
				return result, fmt.Errorf("mark rejected outbox operation failed (%s): %w", reject.OpID, err)
			}
			result.PushedRejected++
		}

		for _, opID := range pushResp.Accepted {
			if _, rejected := rejectedByOpID[opID]; rejected {
				continue
			}
			if _, ok := pendingByOpID[opID]; !ok {
				continue
			}

			if err := c.noteDB.MarkOutboxAcked(ctx, opID); err != nil {
				return result, fmt.Errorf("mark outbox operation acked (%s): %w", opID, err)
			}
			result.PushedAccepted++
		}

		for _, conflict := range pushResp.Conflicts {
			event := mapConflictEvent(conflict, pushResp.ServerCursor, c.cfg.DeviceID, pendingByOpID)

			if err := c.noteDB.InsertSyncConflict(ctx, event); err != nil {
				return result, fmt.Errorf("record local sync conflict: %w", err)
			}
			result.ConflictsLogged++

			if c.artifactWriter != nil {
				if _, err := c.artifactWriter.WriteJSONArtifact(c.cfg.DeviceID, event); err != nil {
					return result, fmt.Errorf("write conflict artifact: %w", err)
				}
				result.ConflictArtifactsWrote++
			}
		}
	}

	currentCursor := cursor
	for {
		pullResp, err := c.pull(ctx, currentCursor)
		if err != nil {
			return result, err
		}

		if len(pullResp.Operations) > 0 {
			if err := c.applyPulledOperations(ctx, pullResp.Operations); err != nil {
				return result, err
			}
			result.PulledApplied += len(pullResp.Operations)
		}

		nextCursor := strings.TrimSpace(pullResp.NextCursor)
		if nextCursor == "" {
			nextCursor = currentCursor
		}

		if nextCursor != currentCursor {
			if err := c.noteDB.SetSyncState(ctx, SyncStateLastServerCursorKey, nextCursor); err != nil {
				return result, fmt.Errorf("persist next cursor %s: %w", nextCursor, err)
			}
			currentCursor = nextCursor
			result.FinalCursor = nextCursor
		}

		if !pullResp.HasMore {
			break
		}

		if pullResp.HasMore && nextCursor == strings.TrimSpace(pullResp.NextCursor) && len(pullResp.Operations) == 0 {
			return result, fmt.Errorf("pull loop cannot progress: has_more=true with no operations")
		}
	}

	result.FinalCursor = currentCursor
	latestCursor, lag, err := c.fetchCursorLag(ctx, currentCursor)
	if err == nil {
		result.ServerLatestCursor = latestCursor
		result.CursorLag = lag
	}

	return result, nil
}

func (c *Client) RunSyncWithRetry(ctx context.Context, cfg RetryConfig) (SyncResult, error) {
	cfg = normalizeRetryConfig(cfg)

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := c.RunSyncOnce(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !isTransientSyncError(err) || attempt == cfg.MaxAttempts {
			return SyncResult{}, err
		}

		delay := nextBackoffDelay(cfg, attempt)
		select {
		case <-ctx.Done():
			return SyncResult{}, ctx.Err()
		case <-time.After(delay):
		}
	}

	if lastErr != nil {
		return SyncResult{}, lastErr
	}

	return SyncResult{}, fmt.Errorf("sync retry loop exited unexpectedly")
}

func (c *Client) RunSyncWorker(ctx context.Context, cfg WorkerConfig) (WorkerResult, error) {
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1
	}
	if cfg.Interval < 0 {
		cfg.Interval = 0
	}

	result := WorkerResult{IterationsRequested: cfg.Iterations}

	for i := 0; i < cfg.Iterations; i++ {
		cycleResult, err := c.RunSyncWithRetry(ctx, cfg.Retry)
		if err != nil {
			return result, fmt.Errorf("sync cycle %d/%d: %w", i+1, cfg.Iterations, err)
		}

		result.IterationsCompleted++
		result.Last = cycleResult
		result.Aggregate = combineSyncResult(result.Aggregate, cycleResult)

		if i == cfg.Iterations-1 || cfg.Interval == 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(cfg.Interval):
		}
	}

	return result, nil
}

func (c *Client) readCursor(ctx context.Context) (string, error) {
	v, err := c.noteDB.GetSyncState(ctx, SyncStateLastServerCursorKey)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "0", nil
	}

	raw := strings.TrimSpace(*v)
	if raw == "" {
		return "0", nil
	}

	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		return "", fmt.Errorf("invalid cursor value %q: %w", raw, err)
	}

	return raw, nil
}

func buildPushOperations(rows []db.SyncOutboxRow) []pushOperation {
	ops := make([]pushOperation, 0, len(rows))
	for _, row := range rows {
		payload := []byte(strings.TrimSpace(row.Payload))
		if len(payload) == 0 {
			payload = []byte("null")
		}
		if !json.Valid(payload) {
			quoted, _ := json.Marshal(row.Payload)
			payload = quoted
		}

		clientUpdated := strings.TrimSpace(row.UpdatedAt)
		if clientUpdated == "" {
			clientUpdated = strings.TrimSpace(row.CreatedAt)
		}

		ops = append(ops, pushOperation{
			OpID:           row.OpID,
			IdempotencyKey: row.IdempotencyKey,
			EntityType:     row.EntityType,
			EntityID:       row.EntityKey,
			OpType:         row.OpType,
			Payload:        payload,
			ClientUpdated:  clientUpdated,
			BaseVersion:    row.BaseVersion,
		})
	}

	return ops
}

func mapConflictEvent(conflict pushConflict, serverCursor string, deviceID string, pendingByOpID map[string]db.SyncOutboxRow) syncops.ConflictEvent {
	entityType := strings.TrimSpace(conflict.EntityType)
	if entityType == "" {
		entityType = string(syncops.EntityNote)
	}

	event := syncops.ConflictEvent{
		EntityType:     syncops.EntityType(entityType),
		EntityKey:      strings.TrimSpace(conflict.EntityID),
		Reason:         strings.TrimSpace(conflict.Reason),
		WinnerDeviceID: strings.TrimSpace(conflict.WinnerDeviceID),
		LoserDeviceID:  strings.TrimSpace(conflict.LoserDeviceID),
		ServerCursor:   strings.TrimSpace(serverCursor),
		OccurredAt:     time.Now().UTC(),
	}

	if event.Reason == "" {
		event.Reason = "base_version_mismatch"
	}

	localOpID := ""
	remoteOpID := ""
	if strings.EqualFold(conflict.WinnerDeviceID, deviceID) {
		localOpID = strings.TrimSpace(conflict.Winner)
		remoteOpID = strings.TrimSpace(conflict.Loser)
	} else if strings.EqualFold(conflict.LoserDeviceID, deviceID) {
		localOpID = strings.TrimSpace(conflict.Loser)
		remoteOpID = strings.TrimSpace(conflict.Winner)
	} else {
		localOpID = strings.TrimSpace(conflict.Loser)
		remoteOpID = strings.TrimSpace(conflict.Winner)
	}

	if row, ok := pendingByOpID[localOpID]; ok {
		event.LocalPayload = row.Payload
		if strings.TrimSpace(event.EntityKey) == "" {
			event.EntityKey = row.EntityKey
		}
		if strings.TrimSpace(string(event.EntityType)) == "" {
			event.EntityType = syncops.EntityType(row.EntityType)
		}
	}

	if row, ok := pendingByOpID[remoteOpID]; ok {
		event.RemotePayload = row.Payload
		if strings.TrimSpace(event.EntityKey) == "" {
			event.EntityKey = row.EntityKey
		}
	}

	return event
}

func (c *Client) push(ctx context.Context, payload pushRequest) (pushResponse, error) {
	resp := pushResponse{}

	body, err := json.Marshal(payload)
	if err != nil {
		return resp, fmt.Errorf("marshal push request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/sync/push", bytes.NewReader(body))
	if err != nil {
		return resp, fmt.Errorf("build push request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("push request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("read push response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resp, &httpStatusError{Operation: "push", StatusCode: httpResp.StatusCode, Body: string(bodyBytes)}
	}

	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return resp, fmt.Errorf("decode push response: %w", err)
	}

	return resp, nil
}

func (c *Client) pull(ctx context.Context, cursor string) (pullResponse, error) {
	resp := pullResponse{}

	cursorValue, err := strconv.ParseInt(strings.TrimSpace(cursor), 10, 64)
	if err != nil {
		return resp, fmt.Errorf("invalid cursor %q: %w", cursor, err)
	}

	url := fmt.Sprintf("%s/v1/sync/pull?cursor=%d&limit=%d", c.cfg.BaseURL, cursorValue, c.cfg.PullLimit)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return resp, fmt.Errorf("build pull request: %w", err)
	}
	if c.cfg.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("pull request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("read pull response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resp, &httpStatusError{Operation: "pull", StatusCode: httpResp.StatusCode, Body: string(bodyBytes)}
	}

	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return resp, fmt.Errorf("decode pull response: %w", err)
	}

	if strings.TrimSpace(resp.NextCursor) == "" {
		resp.NextCursor = cursor
	}

	return resp, nil
}

func (c *Client) applyPulledOperations(ctx context.Context, operations []pullOperation) error {
	for _, op := range operations {
		if err := c.applyOperation(ctx, op); err != nil {
			return fmt.Errorf("apply pulled operation %s: %w", op.OpID, err)
		}
	}
	return nil
}

func (c *Client) applyOperation(ctx context.Context, op pullOperation) error {
	entityType := strings.TrimSpace(op.EntityType)
	opType := strings.TrimSpace(op.OpType)
	entityKey := strings.TrimSpace(op.EntityID)

	if entityKey == "" {
		return fmt.Errorf("entity_id is required")
	}

	if entityType != string(syncops.EntityNote) && entityType != string(syncops.EntityTodo) {
		return fmt.Errorf("unsupported entity_type %q", entityType)
	}

	if err := c.noteDB.WithTx(ctx, func(tx *db.Tx) error {
		switch syncops.EntityType(entityType) {
		case syncops.EntityNote:
			switch opType {
			case string(syncops.OperationUpsert):
				if err := c.applyNoteUpsertTx(tx, op); err != nil {
					return err
				}
			case string(syncops.OperationDelete):
				if err := c.applyNoteDeleteTx(tx, op); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported op_type %q", opType)
			}
		case syncops.EntityTodo:
			switch opType {
			case string(syncops.OperationUpsert):
				if err := c.applyTodoUpsertTx(tx, op); err != nil {
					return err
				}
			case string(syncops.OperationDelete):
				if err := c.applyTodoDeleteTx(tx, op); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported op_type %q", opType)
			}
		}

		_, err := tx.IncrementSyncEntityVersion(syncops.EntityType(entityType), entityKey)
		if err != nil {
			return fmt.Errorf("increment local sync entity version for %s/%s: %w", entityType, entityKey, err)
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}

func (c *Client) applyNoteUpsertTx(tx *db.Tx, op pullOperation) error {
	if len(op.Payload) == 0 {
		return fmt.Errorf("missing payload for note upsert")
	}

	trimmedPayload := strings.TrimSpace(string(op.Payload))
	if trimmedPayload == "" || trimmedPayload == "null" {
		return fmt.Errorf("null payload for note upsert")
	}

	var payload notePayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("decode note payload: %w", err)
	}

	path := strings.TrimSpace(payload.Path)
	if path == "" {
		path = strings.TrimSpace(op.EntityID)
	}
	if path == "" {
		return fmt.Errorf("note path is required")
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}

	parsed := parser.ParseNote(payload.Content, path)

	domains := normalizeUniqueStrings(payload.Domains)
	if len(domains) == 0 {
		domains = normalizeUniqueStrings(parsed.Domains)
	}

	tags := normalizeUniqueStrings(payload.Tags)
	if len(tags) == 0 {
		tags = normalizeUniqueStrings(parsed.Tags)
	}

	linkRows := make([]db.LinkRow, 0, len(parsed.Links))
	for _, link := range parsed.Links {
		linkRows = append(linkRows, db.LinkRow{
			Type:         link.Type,
			Target:       link.Target,
			Label:        link.Label,
			ResolvedPath: link.ResolvedPath,
		})
	}

	noteID, err := tx.UpsertNoteRow(path, title, payload.Content, c.nowFn().UTC())
	if err != nil {
		return fmt.Errorf("upsert note row: %w", err)
	}

	if err := tx.ClearNoteChildren(noteID); err != nil {
		return fmt.Errorf("clear note children: %w", err)
	}

	if err := tx.InsertDomains(noteID, domains); err != nil {
		return fmt.Errorf("insert note domains: %w", err)
	}

	if err := tx.InsertTags(noteID, tags); err != nil {
		return fmt.Errorf("insert note tags: %w", err)
	}

	if err := tx.InsertLinks(noteID, linkRows); err != nil {
		return fmt.Errorf("insert note links: %w", err)
	}

	return nil
}

func (c *Client) applyNoteDeleteTx(tx *db.Tx, op pullOperation) error {
	path := strings.TrimSpace(op.EntityID)
	if path == "" {
		var payload struct {
			Path string `json:"path"`
		}
		if len(op.Payload) > 0 && strings.TrimSpace(string(op.Payload)) != "null" {
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				return fmt.Errorf("decode note delete payload: %w", err)
			}
			path = strings.TrimSpace(payload.Path)
		}
	}

	if path == "" {
		return fmt.Errorf("note delete path is required")
	}

	return tx.DeleteNoteByPath(path)
}

func (c *Client) applyTodoUpsertTx(tx *db.Tx, op pullOperation) error {
	if len(op.Payload) == 0 {
		return fmt.Errorf("missing payload for todo upsert")
	}

	trimmedPayload := strings.TrimSpace(string(op.Payload))
	if trimmedPayload == "" || trimmedPayload == "null" {
		return fmt.Errorf("null payload for todo upsert")
	}

	var payload todoPayload
	if err := json.Unmarshal(op.Payload, &payload); err != nil {
		return fmt.Errorf("decode todo payload: %w", err)
	}

	id := strings.TrimSpace(payload.ID)
	if id == "" {
		id = strings.TrimSpace(op.EntityID)
	}
	if id == "" {
		return fmt.Errorf("todo id is required")
	}

	sourceID := strings.TrimSpace(payload.SourceID)
	if sourceID == "" {
		sourceID = strings.TrimSpace(payload.SourcePath)
	}
	if sourceID == "" {
		sourceID = id
	}

	todoSection := strings.TrimSpace(payload.TodoSection)
	if todoSection == "" {
		todoSection = sectionFromLegacyStatus(payload.LegacyStatus)
	}

	taskText := strings.TrimSpace(payload.Text)
	if taskText == "" {
		taskText = strings.TrimSpace(payload.LegacyTitle)
	}
	if taskText == "" {
		return fmt.Errorf("todo text is required")
	}

	done := payload.Done
	if !done {
		done = strings.EqualFold(strings.TrimSpace(payload.LegacyStatus), "done") || strings.EqualFold(strings.TrimSpace(payload.LegacyStatus), "archived")
	}

	rawPayload := strings.TrimSpace(string(op.Payload))
	if rawPayload == "" {
		rawPayload = "{}"
	}

	var sourcePath *string
	if v := strings.TrimSpace(payload.SourcePath); v != "" {
		sourcePath = &v
	}

	var taskScope *string
	if v := strings.TrimSpace(payload.TaskScope); v != "" {
		taskScope = &v
	}

	var taskArea *string
	if v := strings.TrimSpace(payload.TaskArea); v != "" {
		taskArea = &v
	}

	var meta *string
	if v := strings.TrimSpace(payload.Meta); v != "" {
		meta = &v
	}

	var lineNumber *int
	if payload.Line > 0 {
		line := payload.Line
		lineNumber = &line
	}

	params := db.SyncTodoUpsertParams{
		ID:          id,
		SourceID:    sourceID,
		SourcePath:  sourcePath,
		TaskScope:   taskScope,
		TaskArea:    taskArea,
		TodoSection: todoSection,
		TaskText:    taskText,
		IsDone:      done,
		Meta:        meta,
		TaskOrder:   payload.Order,
		LineNumber:  lineNumber,
		UpdatedAt:   payload.UpdatedAt,
		Payload:     rawPayload,
	}

	if err := tx.UpsertSyncTodo(params); err != nil {
		return fmt.Errorf("upsert sync todo entity: %w", err)
	}

	return nil
}

func (c *Client) applyTodoDeleteTx(tx *db.Tx, op pullOperation) error {
	id := strings.TrimSpace(op.EntityID)
	if id == "" {
		var payload struct {
			ID string `json:"id"`
		}
		if len(op.Payload) > 0 && strings.TrimSpace(string(op.Payload)) != "null" {
			if err := json.Unmarshal(op.Payload, &payload); err != nil {
				return fmt.Errorf("decode todo delete payload: %w", err)
			}
			id = strings.TrimSpace(payload.ID)
		}
	}

	if id == "" {
		return fmt.Errorf("todo delete id is required")
	}

	return tx.DeleteSyncTodo(id)
}

func sectionFromLegacyStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "doing":
		return "Next"
	case "done", "archived":
		return "Waiting"
	default:
		return "Inbox"
	}
}

func (c *Client) fetchCursorLag(ctx context.Context, localCursor string) (string, int64, error) {
	state, err := c.fetchServerSyncState(ctx)
	if err != nil {
		return "", 0, err
	}

	localValue, err := strconv.ParseInt(strings.TrimSpace(localCursor), 10, 64)
	if err != nil {
		return state.LatestCursor, 0, nil
	}

	latestValue, err := strconv.ParseInt(strings.TrimSpace(state.LatestCursor), 10, 64)
	if err != nil {
		return state.LatestCursor, 0, nil
	}

	lag := latestValue - localValue
	if lag < 0 {
		lag = 0
	}

	return state.LatestCursor, lag, nil
}

type syncStateResponse struct {
	ServerTime   string `json:"server_time"`
	LatestCursor string `json:"latest_cursor"`
}

func (c *Client) fetchServerSyncState(ctx context.Context) (syncStateResponse, error) {
	resp := syncStateResponse{}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/v1/sync/state", nil)
	if err != nil {
		return resp, fmt.Errorf("build sync state request: %w", err)
	}
	if c.cfg.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.AuthToken)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return resp, fmt.Errorf("sync state request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return resp, fmt.Errorf("read sync state response: %w", err)
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return resp, &httpStatusError{Operation: "state", StatusCode: httpResp.StatusCode, Body: string(bodyBytes)}
	}

	if err := json.Unmarshal(bodyBytes, &resp); err != nil {
		return resp, fmt.Errorf("decode sync state response: %w", err)
	}

	resp.LatestCursor = strings.TrimSpace(resp.LatestCursor)
	if resp.LatestCursor == "" {
		resp.LatestCursor = "0"
	}

	return resp, nil
}

func combineSyncResult(current SyncResult, next SyncResult) SyncResult {
	combined := SyncResult{}
	combined.PushedAccepted = current.PushedAccepted + next.PushedAccepted
	combined.PushedRejected = current.PushedRejected + next.PushedRejected
	combined.ConflictsLogged = current.ConflictsLogged + next.ConflictsLogged
	combined.ConflictArtifactsWrote = current.ConflictArtifactsWrote + next.ConflictArtifactsWrote
	combined.PulledApplied = current.PulledApplied + next.PulledApplied
	combined.FinalCursor = next.FinalCursor
	combined.ServerLatestCursor = next.ServerLatestCursor
	combined.CursorLag = next.CursorLag
	return combined
}

func normalizeRetryConfig(cfg RetryConfig) RetryConfig {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 500 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 5 * time.Second
	}
	if cfg.MaxDelay < cfg.BaseDelay {
		cfg.MaxDelay = cfg.BaseDelay
	}
	if cfg.JitterRatio < 0 {
		cfg.JitterRatio = 0
	}
	if cfg.JitterRatio > 1 {
		cfg.JitterRatio = 1
	}
	if cfg.JitterRatio == 0 {
		cfg.JitterRatio = 0.2
	}
	return cfg
}

func nextBackoffDelay(cfg RetryConfig, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	factor := math.Pow(2, float64(attempt-1))
	base := float64(cfg.BaseDelay) * factor
	if base > float64(cfg.MaxDelay) {
		base = float64(cfg.MaxDelay)
	}

	jitterWindow := base * cfg.JitterRatio
	if jitterWindow <= 0 {
		return time.Duration(base)
	}

	jitter := (rand.Float64()*2 - 1) * jitterWindow
	delay := base + jitter
	if delay < float64(cfg.BaseDelay) {
		delay = float64(cfg.BaseDelay)
	}
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	return time.Duration(delay)
}

func isTransientSyncError(err error) bool {
	if err == nil {
		return false
	}

	var statusErr *httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		if netErr.Temporary() {
			return true
		}
	}

	message := strings.ToLower(err.Error())
	if strings.Contains(message, "connection refused") ||
		strings.Contains(message, "connection reset") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "eof") {
		return true
	}

	return false
}

func normalizeUniqueStrings(input []string) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(input))

	for _, value := range input {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := set[trimmed]; exists {
			continue
		}
		set[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	sort.Strings(out)
	return out
}
