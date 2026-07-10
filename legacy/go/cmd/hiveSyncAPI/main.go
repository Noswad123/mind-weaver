package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/syncapi"
	_ "github.com/lib/pq"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store := syncapi.Store(syncapi.NewMemoryStore())

	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		db, err := sql.Open("postgres", databaseURL)
		if err != nil {
			log.Fatalf("failed to open postgres connection: %v", err)
		}
		defer db.Close()

		pgStore := syncapi.NewPostgresStore(db)
		if err := pgStore.EnsureSchema(context.Background()); err != nil {
			log.Fatalf("failed to ensure postgres schema: %v", err)
		}

		store = pgStore
		log.Println("🗄️  using Postgres store")
	} else {
		log.Println("⚠️  DATABASE_URL not set, using in-memory store")
	}

	authenticator, authEnabled, err := buildTokenAuthenticatorFromEnv()
	if err != nil {
		log.Fatalf("failed to initialize auth config: %v", err)
	}

	if authEnabled {
		log.Println("🔐 device bearer auth enabled")
	} else {
		log.Println("⚠️  device bearer auth disabled")
	}

	corsConfig := buildCORSConfigFromEnv()
	if corsConfig == nil {
		log.Println("⚠️  CORS disabled (no allowed origins configured)")
	} else {
		log.Printf("🌐 CORS enabled for %d origin(s)", len(corsConfig.AllowedOrigins))
	}

	srv := syncapi.NewServerWithStoreAuthenticatorAndCORS(store, authenticator, corsConfig)
	addr := ":" + port
	log.Printf("🧠 hive-sync-api listening on %s", addr)

	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}

func buildTokenAuthenticatorFromEnv() (*syncapi.TokenAuthenticator, bool, error) {
	requireAuth := parseBoolEnv(os.Getenv("HIVE_SYNC_REQUIRE_AUTH"))
	raw := strings.TrimSpace(os.Getenv("HIVE_SYNC_DEVICE_TOKENS"))

	if raw == "" {
		if requireAuth {
			return nil, false, fmt.Errorf("HIVE_SYNC_REQUIRE_AUTH is enabled but HIVE_SYNC_DEVICE_TOKENS is empty")
		}
		return nil, false, nil
	}

	deviceTokens, err := parseDeviceTokenMap(raw)
	if err != nil {
		return nil, false, err
	}
	if len(deviceTokens) == 0 {
		if requireAuth {
			return nil, false, fmt.Errorf("no valid device tokens parsed from HIVE_SYNC_DEVICE_TOKENS")
		}
		return nil, false, nil
	}

	return syncapi.NewTokenAuthenticator(deviceTokens), true, nil
}

func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func parseDeviceTokenMap(raw string) (map[string]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\t':
			return true
		default:
			return false
		}
	})

	deviceTokens := map[string]string{}
	deviceByToken := map[string]string{}

	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}

		var deviceID string
		var token string

		if strings.Contains(entry, "=") {
			pair := strings.SplitN(entry, "=", 2)
			deviceID = strings.TrimSpace(pair[0])
			token = strings.TrimSpace(pair[1])
		} else if strings.Contains(entry, ":") {
			pair := strings.SplitN(entry, ":", 2)
			deviceID = strings.TrimSpace(pair[0])
			token = strings.TrimSpace(pair[1])
		} else {
			return nil, fmt.Errorf("invalid token entry %q: expected device_id=token", entry)
		}

		if deviceID == "" || token == "" {
			return nil, fmt.Errorf("invalid token entry %q: both device_id and token are required", entry)
		}

		if existing, exists := deviceTokens[deviceID]; exists && existing != token {
			return nil, fmt.Errorf("duplicate device_id %q with conflicting tokens", deviceID)
		}

		if existingDevice, exists := deviceByToken[token]; exists && existingDevice != deviceID {
			return nil, fmt.Errorf("token reused by multiple devices: %q and %q", existingDevice, deviceID)
		}

		deviceTokens[deviceID] = token
		deviceByToken[token] = deviceID
	}

	return deviceTokens, nil
}

func buildCORSConfigFromEnv() *syncapi.CORSConfig {
	raw := strings.TrimSpace(os.Getenv("HIVE_SYNC_CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		return nil
	}

	origins := parseStringList(raw)
	if len(origins) == 0 {
		return nil
	}

	return &syncapi.CORSConfig{AllowedOrigins: origins}
}

func parseStringList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '\n', '\t':
			return true
		default:
			return false
		}
	})

	out := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}

	return out
}
