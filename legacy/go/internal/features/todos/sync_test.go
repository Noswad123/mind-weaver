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

	if !strings.Contains(dashboard, "# Dashboard\n\n## Code") {
		t.Fatalf("expected dashboard level-1 heading with level-2 groups, got:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "## Reading\n- [ ] 35% Sacred Economics [[productivity-beast]]") {
		t.Fatalf("expected reading projection in dashboard, got:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "## Code\n- [ ] Draft architecture review [[productivity-beast]]") {
		t.Fatalf("expected code projection from inline area override, got:\n%s", dashboard)
	}
	if !strings.Contains(dashboard, "## Action\n- [ ] Refill water bottle [[productivity-beast]]") {
		t.Fatalf("expected action projection fallback, got:\n%s", dashboard)
	}
}

func TestListActiveTaskIndexTodos_ReturnsSourceProjection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatalf("mkdir notes dir: %v", err)
	}

	noteContent := `---
id: "productivity-beast"
title: "Productivity Beast"
domains: [task-index]
task_active: true
task_area: Action
---

# productivity beast

## Todo
- [ ] Read one chapter
  - area: Reading
- [x] Ship app fix area:Code
`

	notePath := filepath.Join(notesDir, "hub.md")
	if err := os.WriteFile(notePath, []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	todos, stats, err := ListActiveTaskIndexTodos(notesDir)
	if err != nil {
		t.Fatalf("list active task-index todos: %v", err)
	}

	if stats.ActiveTaskIndexNotes != 1 {
		t.Fatalf("expected 1 active task-index note, got %d", stats.ActiveTaskIndexNotes)
	}
	if len(todos) != 2 {
		t.Fatalf("expected 2 todos, got %d", len(todos))
	}

	if todos[0].SourceID != "productivity-beast" || todos[0].SourcePath != "hub.md" || todos[0].NoteTitle != "Productivity Beast" {
		t.Fatalf("unexpected source fields: %#v", todos[0])
	}
	if todos[0].Area != "Reading" || todos[0].Text != "Read one chapter" || todos[0].Done {
		t.Fatalf("unexpected first todo: %#v", todos[0])
	}
	if todos[1].Area != "Code" || todos[1].Text != "Ship app fix" || !todos[1].Done {
		t.Fatalf("unexpected second todo: %#v", todos[1])
	}
}

func TestToggleTaskIndexTodo_UpdatesSourceAndRefreshesDashboard(t *testing.T) {
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
task_area: Action
---

# productivity beast

## Todo
- [ ] Ship app fix area:Code
`

	notePath := filepath.Join(notesDir, "hub.md")
	if err := os.WriteFile(notePath, []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	dashboardPath := filepath.Join(tmpDir, "dashboard.md")
	if _, err := SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	todos, _, err := ListActiveTaskIndexTodos(notesDir)
	if err != nil {
		t.Fatalf("list active task-index todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	updated, done, err := ToggleTaskIndexTodo(notesDir, dashboardPath, todos[0].ID)
	if err != nil {
		t.Fatalf("toggle todo: %v", err)
	}
	if !done || !updated.Done {
		t.Fatalf("expected toggled todo to be done: %#v", updated)
	}

	sourceBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("read source note: %v", err)
	}
	if !strings.Contains(string(sourceBytes), "- [x] Ship app fix area:Code") {
		t.Fatalf("expected source checkbox to be checked, got:\n%s", string(sourceBytes))
	}

	dashboardBytes, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	if !strings.Contains(string(dashboardBytes), "## Code\n- [x] Ship app fix [[productivity-beast]]") {
		t.Fatalf("expected dashboard to refresh checked projection, got:\n%s", string(dashboardBytes))
	}
}

func TestListActiveTaskIndexTodos_ReturnsStructuredMetadata(t *testing.T) {
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
task_scope: project
task_area: Action
task_default_priority: p4
task_default_energy: small
---

# productivity beast

## Todo
### Next
- [ ] Ship app fix
  - area: Code p2 e:l w:3.5 due:2026-06-20 start:2026-06-18 est:45
`

	if err := os.WriteFile(filepath.Join(notesDir, "hub.md"), []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	todos, _, err := ListActiveTaskIndexTodos(notesDir)
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	meta := todos[0].Metadata
	if todos[0].TaskScope != "project" || todos[0].TodoSection != "Next" || meta.Status != "Next" || meta.TodoSection != "Next" || meta.Area != "Code" || meta.Priority != "p2" || meta.Energy != "l" {
		t.Fatalf("unexpected metadata identity: todo=%#v meta=%#v", todos[0], meta)
	}
	if meta.WeightOverride != "3.5" || meta.Due != "2026-06-20" || meta.Start != "2026-06-18" || meta.Estimate != "45" {
		t.Fatalf("unexpected metadata values: %#v", meta)
	}
}

func TestUpdateTaskIndexTodos_UpdatesMetadataAndDashboard(t *testing.T) {
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
task_area: Action
---

# productivity beast

## Todo
- [ ] Ship app fix
  - p4 e:s due:2026-06-19
`

	notePath := filepath.Join(notesDir, "hub.md")
	if err := os.WriteFile(notePath, []byte(noteContent), 0o644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	dashboardPath := filepath.Join(tmpDir, "dashboard.md")
	if _, err := SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	todos, _, err := ListActiveTaskIndexTodos(notesDir)
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	if len(todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(todos))
	}

	area := "Code"
	priority := "p1"
	energy := "xl"
	due := "2026-06-21"
	est := "90"
	updated, err := UpdateTaskIndexTodos(notesDir, dashboardPath, TodoUpdateParams{
		IDs:      []string{todos[0].ID},
		Area:     &area,
		Priority: &priority,
		Energy:   &energy,
		Due:      &due,
		Estimate: &est,
		Clear:    []string{"weight", "start"},
	})
	if err != nil {
		t.Fatalf("update todos: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("expected 1 updated todo, got %d", len(updated))
	}

	sourceBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !strings.Contains(string(sourceBytes), "  - area: Code p1 e:xl due:2026-06-21 est:90") {
		t.Fatalf("expected canonical metadata line, got:\n%s", string(sourceBytes))
	}

	dashboardBytes, err := os.ReadFile(dashboardPath)
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}
	if !strings.Contains(string(dashboardBytes), "## Code\n- [ ] Ship app fix [[productivity-beast]]") {
		t.Fatalf("expected dashboard to move task to Code, got:\n%s", string(dashboardBytes))
	}
}
