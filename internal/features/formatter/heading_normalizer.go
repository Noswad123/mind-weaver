package formatter

import "strings"

func normalizeMarkdownHeadings(content, title string) (string, bool) {
	lines := strings.Split(content, "\n")
	lines, _ = normalizeMarkdownHeadingLines(lines, title)

	updated := strings.Join(lines, "\n")
	if !strings.HasSuffix(updated, "\n") {
		updated += "\n"
	}

	return updated, updated != content
}

func normalizeMarkdownHeadingLines(lines []string, title string) ([]string, bool) {
	out := append([]string{}, lines...)
	inFence := false
	h1Seen := false
	changed := false

	for i, line := range out {
		trimmed := strings.TrimSpace(line)
		if isFenceDelimiter(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		level, heading, ok := parseHashHeading(trimmed)
		if !ok || level != 1 {
			continue
		}
		if !h1Seen {
			h1Seen = true
			continue
		}

		out[i] = rewriteHeadingLine(line, 2, heading)
		changed = true
	}

	if h1Seen {
		return out, changed
	}

	insertAt := indexAfterFrontmatter(out)
	newTitle := strings.TrimSpace(title)
	if newTitle == "" {
		newTitle = "Untitled"
	}

	rebuilt := make([]string, 0, len(out)+2)
	rebuilt = append(rebuilt, out[:insertAt]...)
	if len(rebuilt) > 0 && strings.TrimSpace(rebuilt[len(rebuilt)-1]) != "" {
		rebuilt = append(rebuilt, "")
	}
	rebuilt = append(rebuilt, "# "+newTitle)
	if insertAt < len(out) && strings.TrimSpace(out[insertAt]) != "" {
		rebuilt = append(rebuilt, "")
	}
	rebuilt = append(rebuilt, out[insertAt:]...)

	return rebuilt, true
}

func indexAfterFrontmatter(lines []string) int {
	idx := 0
	for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
		idx++
	}
	if idx >= len(lines) || strings.TrimSpace(lines[idx]) != "---" {
		return idx
	}

	for i := idx + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			idx = i + 1
			for idx < len(lines) && strings.TrimSpace(lines[idx]) == "" {
				idx++
			}
			return idx
		}
	}

	return idx
}

func rewriteHeadingLine(line string, level int, heading string) string {
	lead := len(line) - len(strings.TrimLeft(line, " 	"))
	if lead < 0 {
		lead = 0
	}
	prefix := line[:lead]
	if strings.TrimSpace(heading) == "" {
		heading = "Untitled"
	}
	return prefix + strings.Repeat("#", level) + " " + strings.TrimSpace(heading)
}

func isFenceDelimiter(trimmed string) bool {
	return strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")
}
