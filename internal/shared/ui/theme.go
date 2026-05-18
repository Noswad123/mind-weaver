package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	Subtle  = lipgloss.Color("#bac2de")
	Accent  = lipgloss.Color("#89dceb")
	Alert   = lipgloss.Color("#f38ba8")
	Checked = lipgloss.Color("#a6e3a1")
	BgDark  = lipgloss.Color("#1e1e2e")
)

var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(Accent).MarginBottom(1)
	LabelStyle  = lipgloss.NewStyle().Width(12).Bold(true)
	AlertStyle  = lipgloss.NewStyle().Foreground(Alert).Italic(true).SetString("!! DANGER !!")
	CursorStyle = lipgloss.NewStyle().Foreground(Accent).Bold(true)

	TabStyle       = lipgloss.NewStyle().Padding(0, 1).Background(BgDark)
	ActiveTabStyle = lipgloss.NewStyle().Padding(0, 1).Background(Accent).Foreground(BgDark)

	ProgressDoneStyle    = lipgloss.NewStyle().Foreground(Checked).Bold(true)
	ProgressPendingStyle = lipgloss.NewStyle().Foreground(Subtle)
	ProgressTickStyle    = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	ProgressDoneLabel    = lipgloss.NewStyle().Foreground(Checked).Bold(true)
	ProgressActiveLabel  = lipgloss.NewStyle().Foreground(Accent).Bold(true)
	ProgressPendingLabel = lipgloss.NewStyle().Foreground(Alert).Bold(true)
	ProgressEmptyLabel   = lipgloss.NewStyle().Foreground(Subtle).Italic(true)
)

type ProgressTask struct {
	Completed bool
	Weight    float64
}

func RenderProgressBar(label string, completed int, total int) string {
	if total < 0 {
		total = 0
	}
	if completed < 0 {
		completed = 0
	}
	if completed > total {
		completed = total
	}

	tasks := make([]ProgressTask, 0, total)
	for i := 0; i < total; i++ {
		tasks = append(tasks, ProgressTask{Completed: i < completed, Weight: 1})
	}

	return RenderWeightedProgressBar(label, tasks)
}

func RenderWeightedProgressBar(label string, tasks []ProgressTask) string {
	completedCount := 0
	for _, task := range tasks {
		if task.Completed {
			completedCount++
		}
	}

	percent := weightedCompletion(tasks)
	bar := renderPartitionedProgressBar(tasks, 25)
	status := renderProgressStatus(completedCount, len(tasks))

	return fmt.Sprintf("%s %s %3.0f%% %s", LabelStyle.Render(label), bar, percent*100, status)
}

func weightedCompletion(tasks []ProgressTask) float64 {
	totalWeight := 0.0
	completedWeight := 0.0

	for _, task := range tasks {
		w := task.Weight
		if w <= 0 {
			w = 1
		}

		totalWeight += w
		if task.Completed {
			completedWeight += w
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return completedWeight / totalWeight
}

func renderPartitionedProgressBar(tasks []ProgressTask, width int) string {
	if width <= 0 {
		return ""
	}
	if len(tasks) == 0 {
		return ProgressPendingStyle.Render(strings.Repeat("░", width))
	}

	weights := make([]float64, len(tasks))
	totalWeight := 0.0
	for i, task := range tasks {
		w := task.Weight
		if w <= 0 {
			w = 1
		}
		weights[i] = w
		totalWeight += w
	}

	boundaryCols := map[int]struct{}{}
	cumulative := 0.0
	for i := 0; i < len(weights)-1; i++ {
		cumulative += weights[i]
		col := int(math.Round((cumulative/totalWeight)*float64(width))) - 1
		if col >= 0 && col < width {
			boundaryCols[col] = struct{}{}
		}
	}

	var b strings.Builder
	taskIndex := 0
	segmentEnd := weights[0]

	for col := 0; col < width; col++ {
		position := (float64(col) + 0.5) / float64(width) * totalWeight
		for taskIndex < len(tasks)-1 && position > segmentEnd {
			taskIndex++
			segmentEnd += weights[taskIndex]
		}

		if _, isBoundary := boundaryCols[col]; isBoundary {
			b.WriteString(ProgressTickStyle.Render("┆"))
			continue
		}

		if tasks[taskIndex].Completed {
			b.WriteString(ProgressDoneStyle.Render("█"))
			continue
		}
		b.WriteString(ProgressPendingStyle.Render("░"))
	}

	return b.String()
}

func renderProgressStatus(completed int, total int) string {
	switch {
	case total == 0:
		return ProgressEmptyLabel.Render("[NO TASKS]")
	case completed == 0:
		return ProgressPendingLabel.Render("[NOT STARTED]")
	case completed == total:
		return ProgressDoneLabel.Render("[COMPLETED]")
	default:
		return ProgressActiveLabel.Render("[IN PROGRESS]")
	}
}
