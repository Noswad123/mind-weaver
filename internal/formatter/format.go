package formatter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/indexer"
	"github.com/Noswad123/mind-weaver/internal/parser"
)

func FormatNote(note parser.ParsedNote, filePath string, notesRoot string) string {
	// If it's a non-root index.norg, ensure it conforms
	if filepath.Base(filePath) == "index.norg" {
		rel, err := filepath.Rel(notesRoot, filePath)
		if err == nil && rel != "index.norg" {
			dir := filepath.Dir(filePath)
			indexer.EnsureIndex(dir, notesRoot)
		}
	}

	var sb strings.Builder

	// Metadata block
	metaLines := []string{}
	if len(note.Tags) > 0 {
		uniqueTags := dedupe(note.Tags)
		sort.Strings(uniqueTags)
		metaLines = append(metaLines, fmt.Sprintf("  tags: [%s]", strings.Join(uniqueTags, ", ")))
	}
	if len(metaLines) > 0 {
		sb.WriteString("@meta\n")
		sb.WriteString(strings.Join(metaLines, "\n"))
		sb.WriteString("\n@end\n\n")
	}

	// Grouped TODOs
	for _, t := range note.Todos {
		if t.IsGroup && t.Task != nil {
			status := fmt.Sprintf("(%s)", t.RawStatus)
			if t.RawStatus == " " {
				status = ""
			}
			prefix := strings.Repeat("*", t.Level)
			line := fmt.Sprintf("%s %s %s", prefix, status, *t.Task)
			sb.WriteString(strings.TrimSpace(line) + "\n")
		} else if !t.IsGroup && t.Task != nil {
			status := fmt.Sprintf("(%s)", t.RawStatus)
			prefix := strings.Repeat("-", t.Depth)
			line := fmt.Sprintf("%s %s %s", prefix, status, *t.Task)
			sb.WriteString(line + "\n")
		}
	}

	sb.WriteString("\n")

	// Preserve content body (non-TODO lines)
	sb.WriteString("---\n")
	sb.WriteString(note.Content)

	return sb.String()
}

func dedupe(items []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, tag := range items {
		if _, ok := seen[tag]; !ok {
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}
