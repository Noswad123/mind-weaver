package todos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncDashboardFromTaskIndexNotes_UsesSubBulletAreaMetadata(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir notes dir: %v", err)
	}

	noteContent := `---
id: "productivity-beast"
domains: [task-index]
task_active: true
task_scope: research
task_area: Action
---

# productivity beast

## Todo
### Inbox
### Next
- [ ] 35% Sacred Economics
  - area: Reading
- [ ] Draft architecture review area:Code
  - area: Reading
- [ ] Refill water bottle
### Waiting
`

	notePath := filepath.Join(notesDir, "hub.md")
	if err := os.WriteFile(notePath, []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	dashboardPath := filepath.Join(tmpDir, "dashboard.md")
	stats, err := SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath)
	if err != nil {
		t.Fatalf("sync dashboard: %v", err)
	}

	if got := stats.TasksByArea["Reading"]; got != 1 {
		t.Fatalf("expected 1 Reading task, got %d", got)
	}
	if got := stats.TasksByArea["Code"]; got != 1 {
		t.Fatalf("expected 1 Code task, got %d", got)
	}
	if got := stats.TasksByArea["Action"]; got != 1 {
		t.Fatalf("expected 1 Action task, got %d", got)
	}

	b, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	dashboard := string(b)

	if !strings.Contains(dashboard, "# Reading\n- [ ] 35% Sacred Economics [[productivity-beast]]") {
		t.Fatalf("expected reading projection in dashboard, got:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "# Code\n- [ ] Draft architecture review [[productivity-beast]]") {
		t.Fatalf("expected code projection from inline area override, got:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "# Action\n- [ ] Refill water bottle [[productivity-beast]]") {
		t.Fatalf("expected action projection fallback, got:\n%s", dashboard)
	}
}
