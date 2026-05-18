package mwcli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestResolveSyncTokenWithRunner_UsesExplicitToken(t *testing.T) {
	t.Parallel()

	called := false
	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	token, err := resolveSyncTokenWithRunner(context.Background(), " abc123 ", "", false, "", "", "desktop", runner)
	if err != nil {
		t.Fatalf("resolve token: %v", err)
	}
	if token != "abc123" {
		t.Fatalf("token=%q want=%q", token, "abc123")
	}
	if called {
		t.Fatalf("runner should not be called when explicit token is provided")
	}
}

func TestResolveSyncTokenWithRunner_UsesTokenCommand(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "/bin/sh" {
			t.Fatalf("runner name=%q want=/bin/sh", name)
		}
		if len(args) != 2 || args[0] != "-c" || !strings.Contains(args[1], "echo") {
			t.Fatalf("unexpected args: %#v", args)
		}
		return []byte("token-from-command\n"), nil
	}

	token, err := resolveSyncTokenWithRunner(context.Background(), "", "echo token-from-command", false, "", "", "desktop", runner)
	if err != nil {
		t.Fatalf("resolve token from command: %v", err)
	}
	if token != "token-from-command" {
		t.Fatalf("token=%q want=%q", token, "token-from-command")
	}
}

func TestResolveSyncTokenWithRunner_UsesKeychain(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("keychain flow is macOS-only")
	}

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "security" {
			t.Fatalf("runner name=%q want=security", name)
		}
		return []byte("token-from-keychain\n"), nil
	}

	token, err := resolveSyncTokenWithRunner(context.Background(), "", "", true, "mw/hive-sync", "desktop", "desktop", runner)
	if err != nil {
		t.Fatalf("resolve token from keychain: %v", err)
	}
	if token != "token-from-keychain" {
		t.Fatalf("token=%q want=%q", token, "token-from-keychain")
	}
}

func TestResolveSyncTokenWithRunner_RejectsMultipleTokenSources(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, nil
	}

	_, err := resolveSyncTokenWithRunner(context.Background(), "", "echo token", true, "", "", "desktop", runner)
	if err == nil {
		t.Fatalf("expected error for conflicting token sources")
	}
}

func TestResolveSyncTokenWithRunner_PropagatesCommandError(t *testing.T) {
	t.Parallel()

	runner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("boom"), errors.New("exit status 1")
	}

	_, err := resolveSyncTokenWithRunner(context.Background(), "", "bad-cmd", false, "", "", "desktop", runner)
	if err == nil {
		t.Fatalf("expected token command error")
	}
}

func TestCheckSyncTokenForDeviceWithClient_Success(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/devices/register" {
			t.Fatalf("path=%q want=/v1/devices/register", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer good-token" {
			t.Fatalf("authorization=%q want=%q", got, "Bearer good-token")
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got, _ := body["device_id"].(string); got != "desktop" {
			t.Fatalf("device_id=%q want=%q", got, "desktop")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"device_id":"desktop","registered":true}`))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	report, err := checkSyncTokenForDeviceWithClient(context.Background(), client, server.URL, "desktop", "good-token")
	if err != nil {
		t.Fatalf("check token: %v", err)
	}
	if !report.Registered {
		t.Fatalf("registered=%v want=true", report.Registered)
	}
	if report.Status != "ok" {
		t.Fatalf("status=%q want=ok", report.Status)
	}
	if report.HTTPStatus != http.StatusOK {
		t.Fatalf("http status=%d want=%d", report.HTTPStatus, http.StatusOK)
	}
}

func TestCheckSyncTokenForDeviceWithClient_Forbidden(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"token device_id does not match request device_id"}`))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	report, err := checkSyncTokenForDeviceWithClient(context.Background(), client, server.URL, "desktop", "bad-token")
	if err != nil {
		t.Fatalf("check token: %v", err)
	}
	if report.Registered {
		t.Fatalf("registered=%v want=false", report.Registered)
	}
	if report.Status != "forbidden" {
		t.Fatalf("status=%q want=forbidden", report.Status)
	}
	if !strings.Contains(report.Message, "token device_id") {
		t.Fatalf("message=%q expected token mismatch detail", report.Message)
	}
}

func TestCheckSyncTokenForDeviceWithClient_Unauthorized(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"missing bearer token"}`))
	}))
	defer server.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	report, err := checkSyncTokenForDeviceWithClient(context.Background(), client, server.URL, "desktop", "some-token")
	if err != nil {
		t.Fatalf("check token: %v", err)
	}
	if report.Status != "unauthorized" {
		t.Fatalf("status=%q want=unauthorized", report.Status)
	}
	if report.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("http status=%d want=%d", report.HTTPStatus, http.StatusUnauthorized)
	}
}
