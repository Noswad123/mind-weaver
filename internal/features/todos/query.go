package todos

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

type TodoResult struct {
	ID          string `json:"id"`
	NoteID      string `json:"noteID"`
	NoteTitle   string `json:"noteTitle"`
	Path        string `json:"path"`
	Title       string `json:"title"`
	IsDone      bool   `json:"isDone"`
	Status      string `json:"status"`
	RawStatus   string `json:"rawStatus"`
	Section     string `json:"section"`
	TaskGroupID string `json:"taskGroupID"`
	Depth       int    `json:"depth"`
	LineNumber  int    `json:"lineNumber"`
}

func QueryTodos(c *cli.Context, svc *Service) error {
	rows, err := svc.ListTodoProjection(c.Context)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	results := make([]TodoResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, TodoResult{
			ID:          strconv.Itoa(row.ID),
			NoteID:      strconv.Itoa(row.NoteID),
			NoteTitle:   row.NoteTitle,
			Path:        row.Path,
			Title:       row.Task,
			IsDone:      isDoneStatus(row.Status, row.RawStatus),
			Status:      row.Status,
			RawStatus:   row.RawStatus,
			Section:     row.TaskGroupName,
			TaskGroupID: strconv.Itoa(row.TaskGroupID),
			Depth:       row.Depth,
			LineNumber:  row.LineNumber,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func isDoneStatus(status, rawStatus string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "done") || strings.EqualFold(strings.TrimSpace(rawStatus), "x")
}
