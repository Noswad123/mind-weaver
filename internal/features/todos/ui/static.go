package ui

import (
	"fmt"
	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	theme "github.com/Noswad123/mind-weaver/internal/shared/ui"
)

func RenderDashboard(todoMap map[string][]parser.Todo, focusGroups []string) error {
	fmt.Println(theme.TitleStyle.Render("CARAMEL"))

	for _, cat := range focusGroups {
		todos := todoMap[cat]

		if len(todos) == 0 {
			fmt.Printf("%s %s\n", theme.LabelStyle.Render(cat), theme.AlertStyle.Render())
			continue
		}

		progressTasks := make([]theme.ProgressTask, 0, len(todos))
		for _, t := range todos {
			progressTasks = append(progressTasks, theme.ProgressTask{Completed: t.IsDone, Weight: t.Weight})
		}

		fmt.Println(theme.RenderWeightedProgressBar(cat, progressTasks))
	}
	return nil
}
