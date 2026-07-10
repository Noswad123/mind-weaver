package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type sourceDefaults struct {
	Priority string
	Energy   string
}

var (
	sourceCheckboxTaskRe = regexp.MustCompile(`^\s*[-*]\s*\[([ xX])\]\s+(.+?)\s*$`)
	inlineAreaTokenRe    = regexp.MustCompile(`(?i)(?:^|\s)area:([A-Za-z][A-Za-z-]*)\b`)
	metaLineTokenRe      = regexp.MustCompile(`(?i)\b(priority:\s*p?[1-5]|energy:\s*[a-zA-Z-]+|(?:w|weight):\s*[0-9]+(?:\.[0-9]+)?|due:\s*\d{4}-\d{2}-\d{2}|start:\s*\d{4}-\d{2}-\d{2}|p:?\s*[1-5]|e:(xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large))\b`)
	bulletPrefixRe       = regexp.MustCompile(`^[*+-]\s+`)
)

func loadTaskIndexSourceContext(notesDir string) (map[string]string, map[string]sourceDefaults, error) {
	sourcePathByID := map[string]string{}
	defaultsByID := map[string]sourceDefaults{}
	root := filepath.Clean(notesDir)

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		meta, ok := readFrontmatterMap(string(b))
		if !ok {
			return nil
		}
		if !hasDomain(meta, "task-index") || !readBool(meta, "task_active") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(filepath.Clean(rel))

		sourceID := strings.TrimSpace(readString(meta, "id"))
		if sourceID == "" {
			sourceID = rel
		}

		sourcePathByID[sourceID] = path
		defaultsByID[sourceID] = sourceDefaults{
			Priority: strings.TrimSpace(readString(meta, "task_default_priority")),
			Energy:   strings.TrimSpace(readString(meta, "task_default_energy")),
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return sourcePathByID, defaultsByID, nil
}

func splitDashboardTaskSourceInfo(todoLine string) (string, string, bool) {
	m := sourceSuffixRe.FindStringSubmatch(todoLine)
	if len(m) < 2 {
		return "", "", false
	}
	sourceID := strings.TrimSpace(m[1])
	taskText := normalizeTaskText(sourceSuffixRe.ReplaceAllString(todoPrefixRe.ReplaceAllString(todoLine, ""), ""))
	if sourceID == "" || taskText == "" {
		return "", "", false
	}
	return sourceID, taskText, true
}

func upsertTaskMetadataInSource(sourcePath string, targetTaskText string, occurrence int, metadata string) error {
	b, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read source note: %w", err)
	}
	lines := strings.Split(string(b), "\n")
	targetTaskText = normalizeTaskText(targetTaskText)

	matchIndexes := []int{}
	inTodo := false
	todoLevel := 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if level, heading, ok := parseHeading(trimmed); ok {
			if strings.EqualFold(heading, "Todo") {
				inTodo = true
				todoLevel = level
			} else if inTodo && level <= todoLevel {
				inTodo = false
				todoLevel = 0
			}
			continue
		}
		if !inTodo {
			continue
		}

		m := sourceCheckboxTaskRe.FindStringSubmatch(trimmed)
		if len(m) == 0 {
			continue
		}
		if normalizeTaskText(m[2]) == targetTaskText {
			matchIndexes = append(matchIndexes, i)
		}
	}

	if len(matchIndexes) == 0 {
		return fmt.Errorf("could not locate task in source note")
	}
	if occurrence <= 0 {
		occurrence = 1
	}
	targetIdx := matchIndexes[0]
	if occurrence-1 < len(matchIndexes) {
		targetIdx = matchIndexes[occurrence-1]
	}

	taskIndent := lineIndent(lines[targetIdx])
	childStart := targetIdx + 1
	childEnd := childStart
	for childEnd < len(lines) {
		trimmed := strings.TrimSpace(lines[childEnd])
		if trimmed == "" {
			childEnd++
			continue
		}
		if _, _, isHeading := parseHeading(trimmed); isHeading {
			break
		}
		if lineIndent(lines[childEnd]) <= taskIndent {
			break
		}
		childEnd++
	}

	keptChildren := make([]string, 0, childEnd-childStart)
	for i := childStart; i < childEnd; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if isTaskMetadataLine(trimmed) {
			continue
		}
		keptChildren = append(keptChildren, lines[i])
	}

	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:childStart]...)
	meta := strings.TrimSpace(metadata)
	if meta != "" {
		metaLine := strings.Repeat(" ", taskIndent+2) + "- " + meta
		out = append(out, metaLine)
	}
	out = append(out, keptChildren...)
	out = append(out, lines[childEnd:]...)

	updated := strings.Join(out, "\n")
	if updated == string(b) {
		return nil
	}
	if err := os.WriteFile(sourcePath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write source note: %w", err)
	}
	return nil
}

func normalizeTaskText(raw string) string {
	cleaned := inlineAreaTokenRe.ReplaceAllString(strings.TrimSpace(raw), " ")
	return strings.Join(strings.Fields(cleaned), " ")
}

func isTaskMetadataLine(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	stripped := strings.TrimSpace(bulletPrefixRe.ReplaceAllString(trimmed, ""))
	if stripped == "" {
		return false
	}
	return metaLineTokenRe.MatchString(stripped)
}

func lineIndent(line string) int {
	indent := 0
	for _, r := range line {
		switch r {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return indent
}

func parseHeading(line string) (int, string, bool) {
	if line == "" {
		return 0, "", false
	}

	marker := byte(0)
	switch line[0] {
	case '#':
		marker = '#'
	case '*':
		marker = '*'
	default:
		return 0, "", false
	}

	i := 0
	for i < len(line) && line[i] == marker {
		i++
	}
	if i == 0 || i >= len(line) || line[i] != ' ' {
		return 0, "", false
	}

	heading := strings.TrimSpace(line[i+1:])
	if heading == "" {
		return 0, "", false
	}
	return i, heading, true
}

func readFrontmatterMap(content string) (map[string]any, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, false
	}

	block := strings.Join(lines[1:end], "\n")
	meta := map[string]any{}
	if err := yaml.Unmarshal([]byte(block), &meta); err != nil {
		return nil, false
	}
	return meta, true
}

func hasDomain(meta map[string]any, wanted string) bool {
	for _, d := range readStringSlice(meta, "domains") {
		if strings.EqualFold(strings.TrimSpace(d), wanted) {
			return true
		}
	}
	return false
}

func readString(meta map[string]any, key string) string {
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func readStringSlice(meta map[string]any, key string) []string {
	v, ok := meta[key]
	if !ok {
		return nil
	}

	out := []string{}
	switch vv := v.(type) {
	case []any:
		for _, item := range vv {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
	case []string:
		for _, item := range vv {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
	case string:
		s := strings.TrimSpace(vv)
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
		for _, part := range strings.Split(s, ",") {
			part = strings.TrimSpace(strings.Trim(part, `"'`))
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func readBool(meta map[string]any, key string) bool {
	v, ok := meta[key]
	if !ok {
		return false
	}

	switch vv := v.(type) {
	case bool:
		return vv
	case int:
		return vv != 0
	case int64:
		return vv != 0
	case float64:
		return vv != 0
	case string:
		s := strings.TrimSpace(strings.ToLower(vv))
		if s == "" {
			return false
		}
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
		return s == "yes" || s == "on" || s == "1"
	default:
		return false
	}
}
