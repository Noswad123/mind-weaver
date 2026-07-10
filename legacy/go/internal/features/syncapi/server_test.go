package syncapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestServer() *Server {
	s := NewServer()
	s.nowFn = func() time.Time {
		return time.Date(2026, time.April, 5, 2, 3, 4, 0, time.UTC)
	}
	return s
}

func newAuthenticatedTestServer() *Server {
	s := NewServerWithStoreAndAuthenticator(NewMemoryStore(), NewTokenAuthenticator(map[string]string{
		"desktop": "token-desktop",
		"phone":   "token-phone",
	}))
	s.nowFn = func() time.Time {
		return time.Date(2026, time.April, 5, 2, 3, 4, 0, time.UTC)
	}
	return s
}

func newCORSTestServer(origins ...string) *Server {
	s := NewServerWithStoreAuthenticatorAndCORS(NewMemoryStore(), nil, &CORSConfig{AllowedOrigins: origins})
	s.nowFn = func() time.Time {
		return time.Date(2026, time.April, 5, 2, 3, 4, 0, time.UTC)
	}
	return s
}

func newAuthenticatedCORSTestServer(origins ...string) *Server {
	s := NewServerWithStoreAuthenticatorAndCORS(NewMemoryStore(), NewTokenAuthenticator(map[string]string{
		"desktop": "token-desktop",
		"phone":   "token-phone",
	}), &CORSConfig{AllowedOrigins: origins})
	s.nowFn = func() time.Time {
		return time.Date(2026, time.April, 5, 2, 3, 4, 0, time.UTC)
	}
	return s
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}
}

func TestSyncStateEndpoint(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/state", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body["latest_cursor"] != "0" {
		t.Fatalf("latest_cursor=%q want=0", body["latest_cursor"])
	}
}

func TestPushEndpoint_RejectsMissingDeviceID(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(`{"operations":[]}`))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
}

func TestPushEndpoint_AcceptsOperationIDs(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	payload := `{
		"device_id": "desktop",
		"operations": [
			{"op_id": "op-1", "idempotency_key": "k1", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert"},
			{"op_id": "op-2", "idempotency_key": "k2", "entity_type": "todo", "entity_id": "todo-1", "op_type": "delete"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(payload))
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var body pushResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Accepted) != 2 {
		t.Fatalf("accepted len=%d want=2", len(body.Accepted))
	}
	if body.ServerCursor != "2" {
		t.Fatalf("server_cursor=%q want=2", body.ServerCursor)
	}
}

func TestPullEndpoint_ValidatesLimit(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/pull?cursor=12&limit=abc", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusBadRequest)
	}
}

func TestPullEndpoint_ReturnsCursorEcho(t *testing.T) {
	t.Parallel()

	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/pull?cursor=12&limit=50", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}

	var body pullResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.NextCursor != "12" {
		t.Fatalf("next_cursor=%q want=12", body.NextCursor)
	}
}

func TestPushAndPull_UsesCursorProgression(t *testing.T) {
	t.Parallel()

	s := newTestServer()

	pushPayload := `{
		"device_id": "desktop",
		"operations": [
			{"op_id": "op-1", "idempotency_key": "k1", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert"},
			{"op_id": "op-2", "idempotency_key": "k2", "entity_type": "note", "entity_id": "notes/todo.md", "op_type": "upsert"}
		]
	}`
	pushReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(pushPayload))
	pushRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(pushRR, pushReq)

	if pushRR.Code != http.StatusOK {
		t.Fatalf("push status=%d want=%d", pushRR.Code, http.StatusOK)
	}

	pullReq := httptest.NewRequest(http.MethodGet, "/v1/sync/pull?cursor=0&limit=1", nil)
	pullRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(pullRR, pullReq)

	if pullRR.Code != http.StatusOK {
		t.Fatalf("pull status=%d want=%d", pullRR.Code, http.StatusOK)
	}

	var pullBody pullResponse
	if err := json.Unmarshal(pullRR.Body.Bytes(), &pullBody); err != nil {
		t.Fatalf("unmarshal pull response: %v", err)
	}
	if len(pullBody.Operations) != 1 {
		t.Fatalf("operations len=%d want=1", len(pullBody.Operations))
	}
	if pullBody.NextCursor != "1" {
		t.Fatalf("next_cursor=%q want=1", pullBody.NextCursor)
	}
	if !pullBody.HasMore {
		t.Fatalf("has_more=false want=true")
	}
}

func TestPush_IdempotencyDedupKeepsCursorStable(t *testing.T) {
	t.Parallel()

	s := newTestServer()

	firstPayload := `{
		"device_id": "desktop",
		"operations": [
			{"op_id": "op-1", "idempotency_key": "same-key", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert"}
		]
	}`
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(firstPayload))
	firstRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(firstRR, firstReq)

	if firstRR.Code != http.StatusOK {
		t.Fatalf("first push status=%d want=%d", firstRR.Code, http.StatusOK)
	}

	secondPayload := `{
		"device_id": "desktop",
		"operations": [
			{"op_id": "op-2", "idempotency_key": "same-key", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert"}
		]
	}`
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(secondPayload))
	secondRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(secondRR, secondReq)

	if secondRR.Code != http.StatusOK {
		t.Fatalf("second push status=%d want=%d", secondRR.Code, http.StatusOK)
	}

	var body pushResponse
	if err := json.Unmarshal(secondRR.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal second response: %v", err)
	}
	if body.ServerCursor != "1" {
		t.Fatalf("server_cursor=%q want=1", body.ServerCursor)
	}
}

func TestPush_ConflictWhenStaleBaseVersionIncomingLoses(t *testing.T) {
	t.Parallel()

	s := newTestServer()

	firstPayload := `{
		"device_id": "work-mac",
		"operations": [
			{"op_id": "op-100", "idempotency_key": "k100", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert", "client_updated_at": "2026-04-05T10:00:00Z", "base_version": 0}
		]
	}`
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(firstPayload))
	firstRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first push status=%d want=%d", firstRR.Code, http.StatusOK)
	}

	secondPayload := `{
		"device_id": "personal-mac",
		"operations": [
			{"op_id": "op-090", "idempotency_key": "k090", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert", "client_updated_at": "2026-04-05T09:00:00Z", "base_version": 0}
		]
	}`
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(secondPayload))
	secondRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("second push status=%d want=%d", secondRR.Code, http.StatusOK)
	}

	var secondBody pushResponse
	if err := json.Unmarshal(secondRR.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("unmarshal second push body: %v", err)
	}
	if len(secondBody.Conflicts) != 1 {
		t.Fatalf("conflicts len=%d want=1", len(secondBody.Conflicts))
	}
	conflict := secondBody.Conflicts[0]
	if conflict.Winner != "op-100" || conflict.Loser != "op-090" {
		t.Fatalf("unexpected conflict winner/loser: %+v", conflict)
	}
	if conflict.WinnerDeviceID != "work-mac" || conflict.LoserDeviceID != "personal-mac" {
		t.Fatalf("unexpected conflict devices: %+v", conflict)
	}

	pullReq := httptest.NewRequest(http.MethodGet, "/v1/sync/pull?cursor=0&limit=10", nil)
	pullRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(pullRR, pullReq)
	if pullRR.Code != http.StatusOK {
		t.Fatalf("pull status=%d want=%d", pullRR.Code, http.StatusOK)
	}

	var pullBody pullResponse
	if err := json.Unmarshal(pullRR.Body.Bytes(), &pullBody); err != nil {
		t.Fatalf("unmarshal pull response: %v", err)
	}
	if len(pullBody.Operations) != 1 {
		t.Fatalf("pull operations len=%d want=1", len(pullBody.Operations))
	}
	if pullBody.Operations[0].OpID != "op-100" {
		t.Fatalf("pull op id=%q want=op-100", pullBody.Operations[0].OpID)
	}
}

func TestPush_ConflictWhenStaleBaseVersionIncomingWins(t *testing.T) {
	t.Parallel()

	s := newTestServer()

	firstPayload := `{
		"device_id": "work-mac",
		"operations": [
			{"op_id": "op-100", "idempotency_key": "k100", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert", "client_updated_at": "2026-04-05T10:00:00Z", "base_version": 0}
		]
	}`
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(firstPayload))
	firstRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first push status=%d want=%d", firstRR.Code, http.StatusOK)
	}

	secondPayload := `{
		"device_id": "personal-mac",
		"operations": [
			{"op_id": "op-200", "idempotency_key": "k200", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert", "client_updated_at": "2026-04-05T11:00:00Z", "base_version": 0}
		]
	}`
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(secondPayload))
	secondRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(secondRR, secondReq)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("second push status=%d want=%d", secondRR.Code, http.StatusOK)
	}

	var secondBody pushResponse
	if err := json.Unmarshal(secondRR.Body.Bytes(), &secondBody); err != nil {
		t.Fatalf("unmarshal second push body: %v", err)
	}
	if len(secondBody.Conflicts) != 1 {
		t.Fatalf("conflicts len=%d want=1", len(secondBody.Conflicts))
	}
	conflict := secondBody.Conflicts[0]
	if conflict.Winner != "op-200" || conflict.Loser != "op-100" {
		t.Fatalf("unexpected conflict winner/loser: %+v", conflict)
	}

	pullReq := httptest.NewRequest(http.MethodGet, "/v1/sync/pull?cursor=0&limit=10", nil)
	pullRR := httptest.NewRecorder()
	s.Handler().ServeHTTP(pullRR, pullReq)
	if pullRR.Code != http.StatusOK {
		t.Fatalf("pull status=%d want=%d", pullRR.Code, http.StatusOK)
	}

	var pullBody pullResponse
	if err := json.Unmarshal(pullRR.Body.Bytes(), &pullBody); err != nil {
		t.Fatalf("unmarshal pull response: %v", err)
	}
	if len(pullBody.Operations) != 2 {
		t.Fatalf("pull operations len=%d want=2", len(pullBody.Operations))
	}
	if pullBody.Operations[1].OpID != "op-200" {
		t.Fatalf("latest pull op id=%q want=op-200", pullBody.Operations[1].OpID)
	}
}

func TestAuth_RejectsMissingBearerToken(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/state", nil)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuth_RejectsInvalidBearerToken(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/state", nil)
	req.Header.Set("Authorization", "Bearer token-unknown")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusForbidden)
	}
}

func TestAuth_AcceptsValidBearerToken(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedTestServer()
	req := httptest.NewRequest(http.MethodGet, "/v1/sync/state", nil)
	req.Header.Set("Authorization", "Bearer token-desktop")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}
}

func TestAuth_PushRejectsDeviceIDTokenMismatch(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedTestServer()
	payload := `{"device_id":"phone","operations":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer token-desktop")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusForbidden)
	}
}

func TestAuth_PushAcceptsMatchingDeviceToken(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedTestServer()
	payload := `{
		"device_id": "desktop",
		"operations": [
			{"op_id": "op-1", "idempotency_key": "k1", "entity_type": "note", "entity_id": "notes/hub.md", "op_type": "upsert"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sync/push", bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer token-desktop")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}
}

func TestCORS_PreflightAllowedOrigin(t *testing.T) {
	t.Parallel()

	s := newCORSTestServer("https://pwa.example.app")
	req := httptest.NewRequest(http.MethodOptions, "/v1/sync/state", nil)
	req.Header.Set("Origin", "https://pwa.example.app")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusNoContent)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://pwa.example.app" {
		t.Fatalf("allow-origin=%q want=%q", got, "https://pwa.example.app")
	}
	if got := rr.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Methods header")
	}
	if got := rr.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatalf("expected Access-Control-Allow-Headers header")
	}
}

func TestCORS_PreflightDisallowedOrigin(t *testing.T) {
	t.Parallel()

	s := newCORSTestServer("https://pwa.example.app")
	req := httptest.NewRequest(http.MethodOptions, "/v1/sync/state", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusForbidden)
	}
}

func TestCORS_PreflightUnknownRoute(t *testing.T) {
	t.Parallel()

	s := newCORSTestServer("https://pwa.example.app")
	req := httptest.NewRequest(http.MethodOptions, "/v1/unknown", nil)
	req.Header.Set("Origin", "https://pwa.example.app")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusNotFound)
	}
}

func TestCORS_SimpleRequestAllowedOriginAddsHeader(t *testing.T) {
	t.Parallel()

	s := newCORSTestServer("https://pwa.example.app")
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://pwa.example.app")
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusOK)
	}
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "https://pwa.example.app" {
		t.Fatalf("allow-origin=%q want=%q", got, "https://pwa.example.app")
	}
}

func TestCORS_PreflightOnAuthenticatedRouteDoesNotRequireAuth(t *testing.T) {
	t.Parallel()

	s := newAuthenticatedCORSTestServer("https://pwa.example.app")
	req := httptest.NewRequest(http.MethodOptions, "/v1/sync/push", nil)
	req.Header.Set("Origin", "https://pwa.example.app")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	rr := httptest.NewRecorder()

	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", rr.Code, http.StatusNoContent)
	}
}
