package syncapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	nowFn func() time.Time
	store Store
	auth  *TokenAuthenticator
	cors  *corsPolicy
	mux   *http.ServeMux
}

type syncOperation struct {
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
	DeviceName string          `json:"device_name"`
	Platform   string          `json:"platform"`
	AppVersion string          `json:"app_version"`
	Operations []syncOperation `json:"operations"`
}

type pushReject struct {
	OpID   string `json:"op_id"`
	Reason string `json:"reason"`
}

type conflictEvent struct {
	EntityType     string `json:"entity_type"`
	EntityID       string `json:"entity_id"`
	Winner         string `json:"winner"`
	Loser          string `json:"loser"`
	Reason         string `json:"reason"`
	WinnerDeviceID string `json:"winner_device_id"`
	LoserDeviceID  string `json:"loser_device_id"`
}

type pushResponse struct {
	Accepted     []string        `json:"accepted"`
	Rejected     []pushReject    `json:"rejected"`
	ServerCursor string          `json:"server_cursor"`
	Conflicts    []conflictEvent `json:"conflicts"`
}

type pullResponse struct {
	Operations []syncOperation `json:"operations"`
	NextCursor string          `json:"next_cursor"`
	HasMore    bool            `json:"has_more"`
}

func NewServer() *Server {
	return NewServerWithStoreAuthenticatorAndCORS(NewMemoryStore(), nil, nil)
}

func NewServerWithStore(store Store) *Server {
	return NewServerWithStoreAuthenticatorAndCORS(store, nil, nil)
}

func NewServerWithStoreAndAuthenticator(store Store, auth *TokenAuthenticator) *Server {
	return NewServerWithStoreAuthenticatorAndCORS(store, auth, nil)
}

func NewServerWithStoreAuthenticatorAndCORS(store Store, auth *TokenAuthenticator, corsConfig *CORSConfig) *Server {
	if store == nil {
		store = NewMemoryStore()
	}

	s := &Server{
		nowFn: time.Now,
		store: store,
		auth:  auth,
		cors:  buildCORSPolicy(corsConfig),
		mux:   http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	if s.cors == nil {
		return s.mux
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			s.mux.ServeHTTP(w, r)
			return
		}

		if !s.cors.isOriginAllowed(origin) {
			if r.Method == http.MethodOptions {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "origin is not allowed"})
				return
			}
			s.mux.ServeHTTP(w, r)
			return
		}

		appendVaryHeader(w, "Origin")
		w.Header().Set("Access-Control-Allow-Origin", s.cors.allowOriginHeaderValue(origin))

		if r.Method == http.MethodOptions {
			appendVaryHeader(w, "Access-Control-Request-Method")
			appendVaryHeader(w, "Access-Control-Request-Headers")

			requestedMethod := strings.TrimSpace(r.Header.Get("Access-Control-Request-Method"))
			if requestedMethod == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing Access-Control-Request-Method"})
				return
			}
			if !s.cors.isMethodAllowed(requestedMethod) {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "requested method is not allowed"})
				return
			}

			preflightRequest := r.Clone(r.Context())
			preflightRequest.Method = requestedMethod
			if _, pattern := s.mux.Handler(preflightRequest); pattern == "" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
				return
			}

			w.Header().Set("Access-Control-Allow-Methods", strings.Join(s.cors.allowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(s.cors.allowedHeaders, ", "))
			w.Header().Set("Access-Control-Max-Age", strconv.Itoa(s.cors.maxAgeSeconds))
			w.WriteHeader(http.StatusNoContent)
			return
		}

		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("GET /v1/sync/state", s.requireAuth(s.handleState))
	s.mux.HandleFunc("POST /v1/sync/push", s.requireAuth(s.handlePush))
	s.mux.HandleFunc("GET /v1/sync/pull", s.requireAuth(s.handlePull))
	s.mux.HandleFunc("POST /v1/devices/register", s.requireAuth(s.handleDeviceRegister))
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	if s.auth == nil {
		return next
	}

	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "Bearer") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization header"})
			return
		}

		deviceID, ok := s.auth.DeviceIDForBearerToken(strings.TrimSpace(parts[1]))
		if !ok {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid bearer token"})
			return
		}

		next(w, r.WithContext(withAuthenticatedDeviceID(r.Context(), deviceID)))
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	latestCursor, err := s.store.LatestCursor(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read sync state"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"server_time":   s.nowFn().UTC().Format(time.RFC3339),
		"latest_cursor": strconv.FormatInt(latestCursor, 10),
	})
}

func (s *Server) handlePush(w http.ResponseWriter, r *http.Request) {
	var req pushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	if authDeviceID, ok := authenticatedDeviceIDFromContext(r.Context()); ok && !strings.EqualFold(authDeviceID, req.DeviceID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "token device_id does not match request device_id"})
		return
	}

	if err := s.store.RegisterDevice(r.Context(), deviceRegistration{
		DeviceID:   req.DeviceID,
		DeviceName: strings.TrimSpace(req.DeviceName),
		Platform:   strings.TrimSpace(req.Platform),
		AppVersion: strings.TrimSpace(req.AppVersion),
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to register device"})
		return
	}

	validOps := make([]syncOperation, 0, len(req.Operations))
	rejected := make([]pushReject, 0)
	for _, op := range req.Operations {
		opID := strings.TrimSpace(op.OpID)
		if opID == "" {
			rejected = append(rejected, pushReject{OpID: "", Reason: "op_id is required"})
			continue
		}

		op.IdempotencyKey = strings.TrimSpace(op.IdempotencyKey)
		if op.IdempotencyKey == "" {
			rejected = append(rejected, pushReject{OpID: opID, Reason: "idempotency_key is required"})
			continue
		}

		op.EntityType = strings.TrimSpace(op.EntityType)
		if !slices.Contains([]string{"note", "todo"}, op.EntityType) {
			rejected = append(rejected, pushReject{OpID: opID, Reason: "entity_type must be note or todo"})
			continue
		}

		op.EntityID = strings.TrimSpace(op.EntityID)
		if op.EntityID == "" {
			rejected = append(rejected, pushReject{OpID: opID, Reason: "entity_id is required"})
			continue
		}

		op.OpType = strings.TrimSpace(op.OpType)
		if !slices.Contains([]string{"upsert", "delete"}, op.OpType) {
			rejected = append(rejected, pushReject{OpID: opID, Reason: "op_type must be upsert or delete"})
			continue
		}

		if op.BaseVersion < 0 {
			rejected = append(rejected, pushReject{OpID: opID, Reason: "base_version must be >= 0"})
			continue
		}

		validOps = append(validOps, op)
	}

	pushResult, err := s.store.Push(r.Context(), req.DeviceID, validOps)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist sync operations"})
		return
	}

	resp := pushResponse{
		Accepted:     pushResult.Accepted,
		Rejected:     rejected,
		ServerCursor: strconv.FormatInt(pushResult.ServerCursor, 10),
		Conflicts:    pushResult.Conflicts,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePull(w http.ResponseWriter, r *http.Request) {
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	if cursor == "" {
		cursor = "0"
	}

	cursorValue, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || cursorValue < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cursor must be a non-negative integer"})
		return
	}

	limit := 100

	if limitRaw := strings.TrimSpace(r.URL.Query().Get("limit")); limitRaw != "" {
		value, err := strconv.Atoi(limitRaw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be an integer"})
			return
		}
		if value <= 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be > 0"})
			return
		}
		if value > 500 {
			value = 500
		}
		limit = value
	}

	operations, nextCursor, hasMore, err := s.store.Pull(r.Context(), cursorValue, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read sync operations"})
		return
	}

	resp := pullResponse{
		Operations: operations,
		NextCursor: strconv.FormatInt(nextCursor, 10),
		HasMore:    hasMore,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleDeviceRegister(w http.ResponseWriter, r *http.Request) {
	var req deviceRegistration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device_id is required"})
		return
	}

	if authDeviceID, ok := authenticatedDeviceIDFromContext(r.Context()); ok && !strings.EqualFold(authDeviceID, req.DeviceID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "token device_id does not match request device_id"})
		return
	}

	if err := s.store.RegisterDevice(r.Context(), req); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to register device"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"device_id":  req.DeviceID,
		"registered": true,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
