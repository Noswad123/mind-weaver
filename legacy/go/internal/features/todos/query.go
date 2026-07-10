package todos

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

type TodoResult struct {
	ID              string       `json:"id"`
	NoteID          string       `json:"noteID"`
	NoteTitle       string       `json:"noteTitle"`
	Path            string       `json:"path"`
	SourceID        string       `json:"sourceID"`
	TaskScope       string       `json:"taskScope"`
	TodoSection     string       `json:"todoSection"`
	Title           string       `json:"title"`
	IsDone          bool         `json:"isDone"`
	Status          string       `json:"status"`
	RawStatus       string       `json:"rawStatus"`
	Section         string       `json:"section"`
	TaskGroupID     string       `json:"taskGroupID"`
	Depth           int          `json:"depth"`
	LineNumber      int          `json:"lineNumber"`
	Metadata        TodoMetadata `json:"metadata"`
	Area            string       `json:"area"`
	Priority        string       `json:"priority"`
	Energy          string       `json:"energy"`
	WeightOverride  string       `json:"weightOverride"`
	Due             string       `json:"due"`
	Start           string       `json:"start"`
	Estimate        string       `json:"estimate"`
	EffectiveWeight float64      `json:"effectiveWeight"`
}

func QueryTodos(c *cli.Context, svc *Service, notesDir string) error {
	activeTodos, _, err := ListActiveTaskIndexTodos(notesDir)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	paths := make([]string, 0, len(activeTodos))
	for _, todo := range activeTodos {
		paths = append(paths, todo.SourcePath)
	}

	notesByPath, err := svc.ListNotesByPaths(c.Context, paths)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	results := make([]TodoResult, 0, len(activeTodos))
	for _, todo := range activeTodos {
		noteID := ""
		noteTitle := todo.NoteTitle
		if noteRow, ok := notesByPath[todo.SourcePath]; ok {
			noteID = strconv.Itoa(int(noteRow.ID))
			if strings.TrimSpace(noteRow.Title) != "" {
				noteTitle = noteRow.Title
			}
		}

		status := todo.Metadata.Status
		if strings.TrimSpace(status) == "" {
			status = todo.TodoSection
		}
		if strings.TrimSpace(status) == "" {
			status = "Inbox"
		}
		rawStatus := " "
		if todo.Done {
			status = "Done"
			rawStatus = "x"
		}

		results = append(results, TodoResult{
			ID:              todo.ID,
			NoteID:          noteID,
			NoteTitle:       noteTitle,
			Path:            todo.SourcePath,
			SourceID:        todo.SourceID,
			TaskScope:       todo.TaskScope,
			TodoSection:     todo.TodoSection,
			Title:           todo.Text,
			IsDone:          todo.Done,
			Status:          status,
			RawStatus:       rawStatus,
			Section:         todo.Area,
			TaskGroupID:     todo.Area,
			Depth:           0,
			LineNumber:      todo.Line,
			Metadata:        todo.Metadata,
			Area:            todo.Metadata.Area,
			Priority:        todo.Metadata.Priority,
			Energy:          todo.Metadata.Energy,
			WeightOverride:  todo.Metadata.WeightOverride,
			Due:             todo.Metadata.Due,
			Start:           todo.Metadata.Start,
			Estimate:        todo.Metadata.Estimate,
			EffectiveWeight: todo.Metadata.EffectiveWeight,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func isDoneStatus(status, rawStatus string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "done") || strings.EqualFold(strings.TrimSpace(rawStatus), "x")
}
