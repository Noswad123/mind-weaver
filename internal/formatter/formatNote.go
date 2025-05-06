package formatter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/parser"
	todoTypes "github.com/Noswad123/mind-weaver/internal/parser/todo"
)

func FormatNote(note parser.ParsedNote, filePath string, notesRoot string) string {
	// If it's a non-root index.norg, ensure it conforms
	if filepath.Base(filePath) == "index.norg" {
		rel, err := filepath.Rel(notesRoot, filePath)
		if err == nil && rel != "index.norg" {
			dir := filepath.Dir(filePath)
			FormatIndexNote(dir, notesRoot)
		}
	}

	var sb strings.Builder

	sb.WriteString(formatMetadata(note.Tags))
	sb.WriteString(formatTodos(note.Todos))

	sb.WriteString(note.Content)

	return sb.String()
}

func formatMetadata(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	uniqueTags := dedupe(tags)
	sort.Strings(uniqueTags)
	return fmt.Sprintf("@meta\n  tags: [%s]\n@end\n\n", strings.Join(uniqueTags, ", "))
}

func formatTodos(todos []todoTypes.Todo) string {
	var sb strings.Builder
	for _, t := range todos {
		if t.Task == nil {
			continue
		}

		status := fmt.Sprintf("(%s)", t.RawStatus)
		if t.RawStatus == " " {
			status = ""
		}

		if t.IsGroup {
			prefix := strings.Repeat("*", t.Level)
			line := fmt.Sprintf("%s %s %s", prefix, status, *t.Task)
			sb.WriteString(strings.TrimSpace(line) + "\n")
		} else {
			prefix := strings.Repeat("-", t.Depth)
			line := fmt.Sprintf("%s %s %s", prefix, status, *t.Task)
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("\n")
	return sb.String()
}

func dedupe(items []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}
