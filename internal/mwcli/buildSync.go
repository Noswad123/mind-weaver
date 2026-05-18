package mwcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/features/syncclient"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
	fsconflicts "github.com/Noswad123/mind-weaver/internal/infra/fs/conflicts"
	"github.com/urfave/cli/v2"
)

func buildSyncCommand(d deps) *cli.Command {
	return &cli.Command{
		Name:  "sync",
		Usage: "Push local outbox, pull remote deltas, and persist cursor",
		Subcommands: []*cli.Command{
			buildSyncDoctorCommand(d),
			buildSyncConflictsCommand(d),
			buildSyncTokenCommand(),
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "endpoint",
				Usage:   "hive-sync-api base URL",
				Value:   defaultSyncEndpoint(),
				EnvVars: []string{"HIVE_SYNC_API_URL"},
			},
			&cli.StringFlag{
				Name:    "device-id",
				Usage:   "Stable device identifier",
				Value:   defaultDeviceID(),
				EnvVars: []string{"HIVE_SYNC_DEVICE_ID"},
			},
			&cli.StringFlag{
				Name:    "device-name",
				Usage:   "Human-friendly device name",
				Value:   defaultDeviceID(),
				EnvVars: []string{"HIVE_SYNC_DEVICE_NAME"},
			},
			&cli.StringFlag{
				Name:    "platform",
				Usage:   "Device platform label",
				Value:   runtime.GOOS,
				EnvVars: []string{"HIVE_SYNC_PLATFORM"},
			},
			&cli.StringFlag{
				Name:    "app-version",
				Usage:   "Client app version sent to sync API",
				Value:   "mw-dev",
				EnvVars: []string{"HIVE_SYNC_APP_VERSION"},
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "Bearer token for authenticated sync endpoints",
				EnvVars: []string{"HIVE_SYNC_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "token-command",
				Usage:   "Shell command that prints bearer token to stdout",
				EnvVars: []string{"HIVE_SYNC_TOKEN_COMMAND"},
			},
			&cli.BoolFlag{
				Name:    "token-from-keychain",
				Usage:   "Load bearer token from macOS Keychain",
				EnvVars: []string{"HIVE_SYNC_TOKEN_FROM_KEYCHAIN"},
			},
			&cli.StringFlag{
				Name:    "token-keychain-service",
				Usage:   "macOS Keychain service name for sync token",
				Value:   defaultSyncTokenKeychainService(),
				EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE"},
			},
			&cli.StringFlag{
				Name:    "token-keychain-account",
				Usage:   "macOS Keychain account name for sync token (defaults to device-id)",
				EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT"},
			},
			&cli.StringFlag{
				Name:    "conflicts-dir",
				Usage:   "Directory for local conflict artifact files",
				Value:   defaultConflictDir(),
				EnvVars: []string{"HIVE_SYNC_CONFLICTS_DIR"},
			},
			&cli.IntFlag{
				Name:    "outbox-limit",
				Usage:   "Max pending outbox operations to push per cycle",
				Value:   100,
				EnvVars: []string{"HIVE_SYNC_OUTBOX_LIMIT"},
			},
			&cli.IntFlag{
				Name:    "pull-limit",
				Usage:   "Max operations to pull per page",
				Value:   100,
				EnvVars: []string{"HIVE_SYNC_PULL_LIMIT"},
			},
			&cli.IntFlag{
				Name:    "retry-attempts",
				Usage:   "Max retry attempts for transient sync failures per cycle",
				Value:   3,
				EnvVars: []string{"HIVE_SYNC_RETRY_ATTEMPTS"},
			},
			&cli.DurationFlag{
				Name:    "retry-base-delay",
				Usage:   "Initial backoff delay for transient retry",
				Value:   500 * time.Millisecond,
				EnvVars: []string{"HIVE_SYNC_RETRY_BASE_DELAY"},
			},
			&cli.DurationFlag{
				Name:    "retry-max-delay",
				Usage:   "Maximum backoff delay for transient retry",
				Value:   5 * time.Second,
				EnvVars: []string{"HIVE_SYNC_RETRY_MAX_DELAY"},
			},
			&cli.IntFlag{
				Name:    "worker-iterations",
				Usage:   "Number of bounded sync cycles to run before exit",
				Value:   1,
				EnvVars: []string{"HIVE_SYNC_WORKER_ITERATIONS"},
			},
			&cli.BoolFlag{
				Name:    "until-empty",
				Usage:   "Run enough sync cycles to drain the currently pending local outbox",
				EnvVars: []string{"HIVE_SYNC_UNTIL_EMPTY"},
			},
			&cli.IntFlag{
				Name:    "until-empty-max-iterations",
				Usage:   "Safety cap for --until-empty calculated sync cycles",
				Value:   100,
				EnvVars: []string{"HIVE_SYNC_UNTIL_EMPTY_MAX_ITERATIONS"},
			},
			&cli.DurationFlag{
				Name:    "worker-interval",
				Usage:   "Delay between worker sync cycles",
				Value:   15 * time.Second,
				EnvVars: []string{"HIVE_SYNC_WORKER_INTERVAL"},
			},
		},
		Action: d.action(func(c *cli.Context, d deps) error {
			return withNoteDb(d.cfg, func(noteDb *db.NoteDb) error {
				writer := fsconflicts.NewArtifactWriter(c.String("conflicts-dir"))
				deviceID := strings.TrimSpace(c.String("device-id"))

				token, err := resolveSyncToken(c.Context, c, deviceID)
				if err != nil {
					return err
				}

				client, err := syncclient.New(noteDb, &http.Client{Timeout: 20 * time.Second}, syncclient.Config{
					BaseURL:         c.String("endpoint"),
					AuthToken:       token,
					DeviceID:        deviceID,
					DeviceName:      c.String("device-name"),
					Platform:        c.String("platform"),
					AppVersion:      c.String("app-version"),
					OutboxBatchSize: c.Int("outbox-limit"),
					PullLimit:       c.Int("pull-limit"),
				}, writer)
				if err != nil {
					return err
				}

				iterations := c.Int("worker-iterations")
				interval := c.Duration("worker-interval")
				if c.Bool("until-empty") {
					diag, err := noteDb.GetSyncDiagnostics(c.Context, syncclient.SyncStateLastServerCursorKey)
					if err != nil {
						return fmt.Errorf("read local sync diagnostics for until-empty: %w", err)
					}

					outboxLimit := c.Int("outbox-limit")
					if outboxLimit <= 0 {
						outboxLimit = 100
					}

					calculatedIterations := 1
					if diag.PendingOutboxCount > 0 {
						calculatedIterations = (diag.PendingOutboxCount + outboxLimit - 1) / outboxLimit
					}

					maxIterations := c.Int("until-empty-max-iterations")
					if maxIterations <= 0 {
						maxIterations = 100
					}
					if calculatedIterations > maxIterations {
						return fmt.Errorf("until-empty needs %d iteration(s) for %d pending op(s) at outbox limit %d; raise --until-empty-max-iterations to continue", calculatedIterations, diag.PendingOutboxCount, outboxLimit)
					}

					iterations = calculatedIterations
					if !c.IsSet("worker-interval") {
						interval = 0
					}
					log.Printf("🧹 draining pending outbox: pending=%d outbox_limit=%d iterations=%d interval=%s", diag.PendingOutboxCount, outboxLimit, iterations, interval)
				}

				workerResult, err := client.RunSyncWorker(c.Context, syncclient.WorkerConfig{
					Iterations: iterations,
					Interval:   interval,
					Retry: syncclient.RetryConfig{
						MaxAttempts: c.Int("retry-attempts"),
						BaseDelay:   c.Duration("retry-base-delay"),
						MaxDelay:    c.Duration("retry-max-delay"),
					},
				})
				if err != nil {
					return err
				}

				result := workerResult.Aggregate
				if workerResult.IterationsCompleted > 0 {
					result = workerResult.Last
				}

				log.Printf("📊 hive sync worker: completed=%d/%d", workerResult.IterationsCompleted, workerResult.IterationsRequested)
				log.Printf("📈 sync counters: pushed=%d rejected=%d pulled=%d conflicts=%d artifacts=%d",
					workerResult.Aggregate.PushedAccepted,
					workerResult.Aggregate.PushedRejected,
					workerResult.Aggregate.PulledApplied,
					workerResult.Aggregate.ConflictsLogged,
					workerResult.Aggregate.ConflictArtifactsWrote,
				)
				log.Printf("📉 conflict rate: %.2f%%", workerResult.Aggregate.ConflictRate()*100)
				if strings.TrimSpace(result.ServerLatestCursor) != "" {
					log.Printf("🧭 cursor state: local=%s server=%s lag=%d", result.FinalCursor, result.ServerLatestCursor, result.CursorLag)
				}

				log.Printf("✅ hive sync complete: pushed=%d rejected=%d pulled=%d conflicts=%d artifacts=%d cursor=%s",
					workerResult.Aggregate.PushedAccepted,
					workerResult.Aggregate.PushedRejected,
					workerResult.Aggregate.PulledApplied,
					workerResult.Aggregate.ConflictsLogged,
					workerResult.Aggregate.ConflictArtifactsWrote,
					result.FinalCursor,
				)

				return nil
			})
		}),
	}
}

type remoteSyncState struct {
	ServerTime   string `json:"server_time"`
	LatestCursor string `json:"latest_cursor"`
}

type syncDoctorReport struct {
	GeneratedAt string             `json:"generated_at"`
	NotesDBPath string             `json:"notes_db_path"`
	Status      string             `json:"status"`
	Warnings    []string           `json:"warnings,omitempty"`
	Local       db.SyncDiagnostics `json:"local"`
	Remote      *remoteSyncState   `json:"remote,omitempty"`
	CursorLag   *int64             `json:"cursor_lag,omitempty"`
}

type syncTokenCheckReport struct {
	GeneratedAt string `json:"generated_at"`
	Endpoint    string `json:"endpoint"`
	DeviceID    string `json:"device_id"`
	Status      string `json:"status"`
	HTTPStatus  int    `json:"http_status"`
	Registered  bool   `json:"registered"`
	Message     string `json:"message,omitempty"`
}

type syncCommandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

var syncRunner syncCommandRunner = defaultSyncCommandRunner

func defaultSyncCommandRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func buildSyncDoctorCommand(d deps) *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Inspect local sync health and cursor state",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Usage:   "Output format: text|json",
				Value:   "text",
				EnvVars: []string{"HIVE_SYNC_DOCTOR_FORMAT"},
			},
			&cli.StringFlag{
				Name:    "endpoint",
				Usage:   "hive-sync-api base URL for remote cursor check",
				Value:   defaultSyncEndpoint(),
				EnvVars: []string{"HIVE_SYNC_API_URL"},
			},
			&cli.StringFlag{
				Name:    "token",
				Usage:   "Bearer token for remote sync-state endpoint",
				EnvVars: []string{"HIVE_SYNC_TOKEN"},
			},
			&cli.StringFlag{
				Name:    "token-command",
				Usage:   "Shell command that prints bearer token to stdout",
				EnvVars: []string{"HIVE_SYNC_TOKEN_COMMAND"},
			},
			&cli.BoolFlag{
				Name:    "token-from-keychain",
				Usage:   "Load bearer token from macOS Keychain",
				EnvVars: []string{"HIVE_SYNC_TOKEN_FROM_KEYCHAIN"},
			},
			&cli.StringFlag{
				Name:    "token-keychain-service",
				Usage:   "macOS Keychain service name for sync token",
				Value:   defaultSyncTokenKeychainService(),
				EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE"},
			},
			&cli.StringFlag{
				Name:    "token-keychain-account",
				Usage:   "macOS Keychain account name for sync token (defaults to device-id)",
				EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT"},
			},
			&cli.StringFlag{
				Name:    "device-id",
				Usage:   "Device identifier used for keychain token account default",
				Value:   defaultDeviceID(),
				EnvVars: []string{"HIVE_SYNC_DEVICE_ID"},
			},
			&cli.DurationFlag{
				Name:    "timeout",
				Usage:   "Remote sync-state request timeout",
				Value:   5 * time.Second,
				EnvVars: []string{"HIVE_SYNC_DOCTOR_TIMEOUT"},
			},
			&cli.BoolFlag{
				Name:    "skip-remote",
				Usage:   "Skip remote sync-state request and only inspect local DB",
				EnvVars: []string{"HIVE_SYNC_DOCTOR_SKIP_REMOTE"},
			},
		},
		Action: d.action(func(c *cli.Context, d deps) error {
			return withNoteDb(d.cfg, func(noteDB *db.NoteDb) error {
				diag, err := noteDB.GetSyncDiagnostics(c.Context, syncclient.SyncStateLastServerCursorKey)
				if err != nil {
					return fmt.Errorf("read local sync diagnostics: %w", err)
				}

				report := syncDoctorReport{
					GeneratedAt: time.Now().UTC().Format(time.RFC3339),
					NotesDBPath: d.cfg.notesDBPath,
					Local:       diag,
					Status:      "healthy",
				}

				warnings := make([]string, 0)
				if diag.PendingOutboxCount > 0 {
					warnings = append(warnings, fmt.Sprintf("pending outbox operations: %d", diag.PendingOutboxCount))
				}
				if diag.PendingOutboxRetriedCount > 0 {
					warnings = append(warnings, fmt.Sprintf("pending outbox retries: %d (max attempts %d)", diag.PendingOutboxRetriedCount, diag.PendingOutboxMaxAttemptCount))
				}
				if diag.UnresolvedConflictCount > 0 {
					warnings = append(warnings, fmt.Sprintf("unresolved conflicts: %d", diag.UnresolvedConflictCount))
				}

				if !c.Bool("skip-remote") {
					token, err := resolveSyncToken(c.Context, c, c.String("device-id"))
					if err != nil {
						return err
					}

					remote, err := fetchRemoteSyncState(c.Context, c.String("endpoint"), token, c.Duration("timeout"))
					if err != nil {
						warnings = append(warnings, fmt.Sprintf("remote sync-state check failed: %v", err))
					} else {
						report.Remote = &remote

						localCursor, localErr := strconv.ParseInt(strings.TrimSpace(diag.LocalCursor), 10, 64)
						remoteCursor, remoteErr := strconv.ParseInt(strings.TrimSpace(remote.LatestCursor), 10, 64)
						if localErr == nil && remoteErr == nil {
							lag := remoteCursor - localCursor
							if lag < 0 {
								lag = 0
							}
							report.CursorLag = &lag
							if lag > 0 {
								warnings = append(warnings, fmt.Sprintf("local cursor lag: %d", lag))
							}
						}
					}
				}

				report.Warnings = warnings
				if len(warnings) > 0 {
					report.Status = "attention"
				}

				switch strings.ToLower(strings.TrimSpace(c.String("format"))) {
				case "json":
					b, err := json.MarshalIndent(report, "", "  ")
					if err != nil {
						return fmt.Errorf("marshal doctor report json: %w", err)
					}
					fmt.Println(string(b))
					return nil
				case "text", "":
					printSyncDoctorTextReport(report)
					return nil
				default:
					return fmt.Errorf("unsupported format %q (expected text|json)", c.String("format"))
				}
			})
		}),
	}
}

func fetchRemoteSyncState(ctx context.Context, endpoint, token string, timeout time.Duration) (remoteSyncState, error) {
	state := remoteSyncState{}

	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return state, fmt.Errorf("endpoint is required for remote check")
	}

	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	httpClient := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/v1/sync/state", nil)
	if err != nil {
		return state, err
	}

	token = strings.TrimSpace(token)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return state, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return state, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return state, fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.Unmarshal(body, &state); err != nil {
		return state, err
	}

	state.LatestCursor = strings.TrimSpace(state.LatestCursor)
	if state.LatestCursor == "" {
		state.LatestCursor = "0"
	}

	return state, nil
}

func checkSyncTokenForDevice(ctx context.Context, endpoint, deviceID, token string, timeout time.Duration) (syncTokenCheckReport, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	return checkSyncTokenForDeviceWithClient(ctx, httpClient, endpoint, deviceID, token)
}

func checkSyncTokenForDeviceWithClient(ctx context.Context, httpClient *http.Client, endpoint, deviceID, token string) (syncTokenCheckReport, error) {
	report := syncTokenCheckReport{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Endpoint:    strings.TrimSpace(endpoint),
		DeviceID:    strings.TrimSpace(deviceID),
		Status:      "error",
	}

	if report.Endpoint == "" {
		return report, fmt.Errorf("endpoint is required")
	}
	if report.DeviceID == "" {
		return report, fmt.Errorf("device_id is required")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return report, fmt.Errorf("token is required")
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	payloadBytes, err := json.Marshal(map[string]string{"device_id": report.DeviceID})
	if err != nil {
		return report, fmt.Errorf("marshal token check payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(report.Endpoint, "/")+"/v1/devices/register", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return report, fmt.Errorf("build token check request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return report, fmt.Errorf("token check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return report, fmt.Errorf("read token check response: %w", err)
	}

	report.HTTPStatus = resp.StatusCode
	message := extractSyncAPIErrorMessage(body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		report.Status = "ok"
		report.Registered = true
		report.Message = "token is valid for device_id"

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err == nil {
			if registered, ok := payload["registered"].(bool); ok {
				report.Registered = registered
			}
		}

		if !report.Registered {
			report.Status = "attention"
			report.Message = "registration response did not confirm token/device match"
		}

		return report, nil
	}

	report.Registered = false
	report.Message = message
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		report.Status = "unauthorized"
	case http.StatusForbidden:
		report.Status = "forbidden"
	default:
		report.Status = "error"
	}

	return report, nil
}

func extractSyncAPIErrorMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if errorValue, ok := payload["error"].(string); ok && strings.TrimSpace(errorValue) != "" {
			return strings.TrimSpace(errorValue)
		}
	}

	return trimmed
}

func printSyncDoctorTextReport(report syncDoctorReport) {
	fmt.Printf("🩺 sync doctor (%s)\n", report.GeneratedAt)
	fmt.Printf("status: %s\n", report.Status)
	fmt.Printf("notes db: %s\n", report.NotesDBPath)

	fmt.Printf("\nLocal\n")
	fmt.Printf("  cursor: %s\n", report.Local.LocalCursor)
	fmt.Printf("  outbox pending: %d\n", report.Local.PendingOutboxCount)
	fmt.Printf("  outbox retries: %d (max attempts %d)\n", report.Local.PendingOutboxRetriedCount, report.Local.PendingOutboxMaxAttemptCount)
	if report.Local.PendingOutboxOldestCreatedAt != "" {
		fmt.Printf("  oldest pending op: %s\n", report.Local.PendingOutboxOldestCreatedAt)
	}
	if report.Local.PendingOutboxLatestFailure != "" {
		fmt.Printf("  latest pending error: %s\n", report.Local.PendingOutboxLatestFailure)
	}
	fmt.Printf("  outbox acked: %d\n", report.Local.AckedOutboxCount)
	fmt.Printf("  conflicts unresolved: %d / total: %d\n", report.Local.UnresolvedConflictCount, report.Local.TotalConflictCount)
	if report.Local.OldestUnresolvedConflictAt != "" {
		fmt.Printf("  oldest unresolved conflict: %s\n", report.Local.OldestUnresolvedConflictAt)
	}
	fmt.Printf("  entity versions tracked: %d\n", report.Local.SyncEntityVersionCount)
	fmt.Printf("  synced todos rows: %d\n", report.Local.SyncedTodoCount)

	if report.Remote != nil {
		fmt.Printf("\nRemote\n")
		fmt.Printf("  latest cursor: %s\n", report.Remote.LatestCursor)
		if strings.TrimSpace(report.Remote.ServerTime) != "" {
			fmt.Printf("  server time: %s\n", report.Remote.ServerTime)
		}
	}

	if report.CursorLag != nil {
		fmt.Printf("\nCursor lag\n")
		fmt.Printf("  lag: %d\n", *report.CursorLag)
	}

	if len(report.Warnings) > 0 {
		fmt.Printf("\nWarnings\n")
		for _, warning := range report.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	} else {
		fmt.Printf("\nWarnings\n")
		fmt.Printf("  - none\n")
	}
}

func buildSyncTokenCommand() *cli.Command {
	return &cli.Command{
		Name:  "token",
		Usage: "Manage sync bearer token storage",
		Subcommands: []*cli.Command{
			{
				Name:  "store",
				Usage: "Store bearer token in macOS Keychain",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "token",
						Usage:   "Bearer token value to store",
						EnvVars: []string{"HIVE_SYNC_TOKEN"},
					},
					&cli.BoolFlag{
						Name:  "token-stdin",
						Usage: "Read bearer token from stdin",
					},
					&cli.StringFlag{
						Name:    "device-id",
						Usage:   "Device identifier used as default keychain account",
						Value:   defaultDeviceID(),
						EnvVars: []string{"HIVE_SYNC_DEVICE_ID"},
					},
					&cli.StringFlag{
						Name:    "token-keychain-service",
						Usage:   "macOS Keychain service name",
						Value:   defaultSyncTokenKeychainService(),
						EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE"},
					},
					&cli.StringFlag{
						Name:    "token-keychain-account",
						Usage:   "macOS Keychain account name (defaults to device-id)",
						EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT"},
					},
				},
				Action: func(c *cli.Context) error {
					token := strings.TrimSpace(c.String("token"))
					if c.Bool("token-stdin") {
						stdinToken, err := io.ReadAll(os.Stdin)
						if err != nil {
							return fmt.Errorf("read token from stdin: %w", err)
						}
						token = strings.TrimSpace(string(stdinToken))
					}

					if token == "" {
						return fmt.Errorf("token is required (set --token or --token-stdin)")
					}

					service := strings.TrimSpace(c.String("token-keychain-service"))
					if service == "" {
						service = defaultSyncTokenKeychainService()
					}

					account := strings.TrimSpace(c.String("token-keychain-account"))
					if account == "" {
						account = strings.TrimSpace(c.String("device-id"))
					}
					if account == "" {
						return fmt.Errorf("keychain account is required (set --token-keychain-account or --device-id)")
					}

					if err := storeTokenInKeychain(c.Context, service, account, token); err != nil {
						return err
					}

					fmt.Printf("✅ stored sync token in Keychain service=%q account=%q\n", service, account)
					fmt.Printf("💡 use with: mw sync --device-id %q --token-from-keychain\n", account)
					return nil
				},
			},
			{
				Name:  "check",
				Usage: "Verify bearer token matches device_id on sync API",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "endpoint",
						Usage:   "hive-sync-api base URL",
						Value:   defaultSyncEndpoint(),
						EnvVars: []string{"HIVE_SYNC_API_URL"},
					},
					&cli.StringFlag{
						Name:    "device-id",
						Usage:   "Device identifier to validate against bearer token",
						Value:   defaultDeviceID(),
						EnvVars: []string{"HIVE_SYNC_DEVICE_ID"},
					},
					&cli.StringFlag{
						Name:    "token",
						Usage:   "Bearer token value",
						EnvVars: []string{"HIVE_SYNC_TOKEN"},
					},
					&cli.StringFlag{
						Name:    "token-command",
						Usage:   "Shell command that prints bearer token to stdout",
						EnvVars: []string{"HIVE_SYNC_TOKEN_COMMAND"},
					},
					&cli.BoolFlag{
						Name:    "token-from-keychain",
						Usage:   "Load bearer token from macOS Keychain",
						EnvVars: []string{"HIVE_SYNC_TOKEN_FROM_KEYCHAIN"},
					},
					&cli.StringFlag{
						Name:    "token-keychain-service",
						Usage:   "macOS Keychain service name for sync token",
						Value:   defaultSyncTokenKeychainService(),
						EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_SERVICE"},
					},
					&cli.StringFlag{
						Name:    "token-keychain-account",
						Usage:   "macOS Keychain account name for sync token (defaults to device-id)",
						EnvVars: []string{"HIVE_SYNC_TOKEN_KEYCHAIN_ACCOUNT"},
					},
					&cli.DurationFlag{
						Name:    "timeout",
						Usage:   "Token validation request timeout",
						Value:   5 * time.Second,
						EnvVars: []string{"HIVE_SYNC_TOKEN_CHECK_TIMEOUT"},
					},
					&cli.StringFlag{
						Name:    "format",
						Usage:   "Output format: text|json",
						Value:   "text",
						EnvVars: []string{"HIVE_SYNC_TOKEN_CHECK_FORMAT"},
					},
				},
				Action: func(c *cli.Context) error {
					deviceID := strings.TrimSpace(c.String("device-id"))
					if deviceID == "" {
						return fmt.Errorf("device-id is required")
					}

					token, err := resolveSyncToken(c.Context, c, deviceID)
					if err != nil {
						return err
					}
					if strings.TrimSpace(token) == "" {
						return fmt.Errorf("token is required (set --token, --token-command, or --token-from-keychain)")
					}

					report, err := checkSyncTokenForDevice(c.Context, c.String("endpoint"), deviceID, token, c.Duration("timeout"))
					if err != nil {
						return err
					}

					switch strings.ToLower(strings.TrimSpace(c.String("format"))) {
					case "json":
						b, err := json.MarshalIndent(report, "", "  ")
						if err != nil {
							return fmt.Errorf("marshal token check report: %w", err)
						}
						fmt.Println(string(b))
					case "text", "":
						fmt.Printf("🔐 sync token check (%s)\n", report.GeneratedAt)
						fmt.Printf("endpoint:    %s\n", report.Endpoint)
						fmt.Printf("device_id:   %s\n", report.DeviceID)
						fmt.Printf("http status: %d\n", report.HTTPStatus)
						fmt.Printf("status:      %s\n", report.Status)
						if strings.TrimSpace(report.Message) != "" {
							fmt.Printf("message:     %s\n", report.Message)
						}
					default:
						return fmt.Errorf("unsupported format %q (expected text|json)", c.String("format"))
					}

					if !report.Registered {
						return fmt.Errorf("token check failed for device_id=%q (status=%s http=%d)", report.DeviceID, report.Status, report.HTTPStatus)
					}

					return nil
				},
			},
		},
	}
}

func resolveSyncToken(ctx context.Context, c *cli.Context, deviceID string) (string, error) {
	return resolveSyncTokenWithRunner(
		ctx,
		strings.TrimSpace(c.String("token")),
		strings.TrimSpace(c.String("token-command")),
		c.Bool("token-from-keychain"),
		strings.TrimSpace(c.String("token-keychain-service")),
		strings.TrimSpace(c.String("token-keychain-account")),
		strings.TrimSpace(deviceID),
		syncRunner,
	)
}

func resolveSyncTokenWithRunner(
	ctx context.Context,
	rawToken string,
	tokenCommand string,
	tokenFromKeychain bool,
	keychainService string,
	keychainAccount string,
	deviceID string,
	runner syncCommandRunner,
) (string, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken != "" {
		return rawToken, nil
	}

	tokenCommand = strings.TrimSpace(tokenCommand)
	if tokenFromKeychain && tokenCommand != "" {
		return "", fmt.Errorf("choose only one token source: --token-command or --token-from-keychain")
	}

	if tokenFromKeychain {
		if runtime.GOOS != "darwin" {
			return "", fmt.Errorf("--token-from-keychain is only supported on macOS")
		}

		service := strings.TrimSpace(keychainService)
		if service == "" {
			service = defaultSyncTokenKeychainService()
		}

		account := strings.TrimSpace(keychainAccount)
		if account == "" {
			account = strings.TrimSpace(deviceID)
		}
		if account == "" {
			return "", fmt.Errorf("token keychain account is required (set --token-keychain-account or --device-id)")
		}

		token, err := fetchTokenFromKeychain(ctx, service, account, runner)
		if err != nil {
			return "", err
		}
		return token, nil
	}

	if tokenCommand != "" {
		out, err := runner(ctx, "/bin/sh", "-c", tokenCommand)
		if err != nil {
			return "", fmt.Errorf("token-command failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
		}

		token := strings.TrimSpace(string(out))
		if token == "" {
			return "", fmt.Errorf("token-command produced empty output")
		}

		return token, nil
	}

	return "", nil
}

func fetchTokenFromKeychain(ctx context.Context, service, account string, runner syncCommandRunner) (string, error) {
	out, err := runner(ctx, "security", "find-generic-password", "-w", "-s", service, "-a", account)
	if err != nil {
		return "", fmt.Errorf("read token from keychain service=%q account=%q: %w (output: %s)", service, account, err, strings.TrimSpace(string(out)))
	}

	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("empty token from keychain service=%q account=%q", service, account)
	}

	return token, nil
}

func storeTokenInKeychain(ctx context.Context, service, account, token string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("keychain token storage is only supported on macOS")
	}

	service = strings.TrimSpace(service)
	if service == "" {
		service = defaultSyncTokenKeychainService()
	}

	account = strings.TrimSpace(account)
	if account == "" {
		return fmt.Errorf("keychain account is required")
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("token is required")
	}

	out, err := syncRunner(ctx, "security", "add-generic-password", "-U", "-s", service, "-a", account, "-w", token)
	if err != nil {
		return fmt.Errorf("store token in keychain service=%q account=%q: %w (output: %s)", service, account, err, strings.TrimSpace(string(out)))
	}

	return nil
}

func defaultSyncTokenKeychainService() string {
	return "mw/hive-sync"
}

func defaultSyncEndpoint() string {
	v := strings.TrimSpace(os.Getenv("HIVE_SYNC_API_URL"))
	if v == "" {
		return "http://127.0.0.1:8080"
	}
	return v
}

func defaultDeviceID() string {
	v := strings.TrimSpace(os.Getenv("HIVE_SYNC_DEVICE_ID"))
	if v != "" {
		return v
	}
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return "mw-device"
	}
	return host
}

func defaultConflictDir() string {
	v := strings.TrimSpace(os.Getenv("HIVE_SYNC_CONFLICTS_DIR"))
	if v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return ".mw-conflicts"
	}
	return filepath.Join(home, ".local", "share", "mw", "conflicts")
}
