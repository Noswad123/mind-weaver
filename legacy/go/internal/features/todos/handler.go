package todos

import (
	"fmt"
	"os"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	tui "github.com/Noswad123/mind-weaver/internal/features/todos/ui"
)

var focusGroups = []string{
	"Code",
	"Action",
	"Reading",
	"Amusement",
	"Music",
	"Exercise",
	"Love",
}

func ViewDashboard(isInteractive bool, notesDir, dashboardPath, inboxPath string) (bool, error) {
	content, err := os.ReadFile(dashboardPath)
	if err != nil {
		return false, fmt.Errorf("could not read dashboard file at %s: %w", dashboardPath, err)
	}

	noteParser := parser.MarkdownParser{}
	todoMap := noteParser.ParseDashboard(string(content), focusGroups)

	defaultsBySource, metadataByTaskKey, err := loadTaskIndexWeightContext(notesDir)
	if err != nil {
		return false, err
	}
	applyTodoWeights(todoMap, defaultsBySource, metadataByTaskKey)

	if isInteractive {
		changed, archiveKeys, err := tui.RunTodoTUI(dashboardPath, notesDir, inboxPath, todoMap, focusGroups)
		if err != nil {
			return false, err
		}
		if len(archiveKeys) > 0 {
			if _, err := ArchiveSelectedToLifeLog(notesDir, archiveKeys); err != nil {
				return false, err
			}
			changed = true
		}
		return changed, nil
	}

	if err := tui.RenderDashboard(todoMap, focusGroups); err != nil {
		return false, err
	}
	return false, nil
}
