package todos

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type SyncStats struct {
	ScannedMarkdownNotes int
	ActiveTaskIndexNotes int
	SyncedTasks          int
	TasksByArea          map[string]int
	SourceWritebacks     int
	SourceFilesUpdated   int
}

type sourcedTask struct {
	Area       string
	Text       string
	Done       bool
	SourcePath string
	SourceID   string
	Order      int
}

type parsedTask struct {
	Text  string
	Done  bool
	Order int
	Line  int
	Meta  string
}

var (
	checkboxTaskRe          = regexp.MustCompile(`^\s*[-*]\s*\[([ xX])\]\s+(.+?)\s*$`)
	inlineAreaRe            = regexp.MustCompile(`(?i)(?:^|\s)area:([A-Za-z][A-Za-z-]*)\b`)
	metadataAreaTokenRe     = regexp.MustCompile(`(?i)\barea:\s*([A-Za-z][A-Za-z-]*)\b`)
	dashboardSourceSuffixRe = regexp.MustCompile(`\s+\[\[([^\]]+)\]\]\s*$`)
	checkboxStateRe         = regexp.MustCompile(`\[[ xX]\]`)
)

func SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath string) (SyncStats, error) {
	stats := SyncStats{TasksByArea: map[string]int{}}
	root := filepath.Clean(notesDir)
	tasks := []sourcedTask{}

	dashboardSelections, err := readDashboardSelections(dashboardPath)
	if err != nil {
		return stats, err
	}
	selectionCursor := map[string]int{}

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		isInbox := strings.HasSuffix(path, "introspection/inbox.md")

		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		stats.ScannedMarkdownNotes++

		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		content := string(b)

		meta, hasMeta := readFrontmatterMap(content)

		shouldProcess := false
		if hasMeta && hasDomain(meta, "task-index") && readBool(meta, "task_active") {
			shouldProcess = true
			stats.ActiveTaskIndexNotes++
		} else if isInbox && !hasMeta {
			shouldProcess = true
		}

		if !shouldProcess {
			return nil
		}

		noteArea := resolveArea(readString(meta, "task_area"), "Action")

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(filepath.Clean(rel))

		sourceID := strings.TrimSpace(readString(meta, "id"))
		if sourceID == "" {
			sourceID = rel
		}

		parsed := extractTodoTasks(content)
		if len(parsed) == 0 {
			return nil
		}

		lines := strings.Split(content, "\n")
		noteChanged := false

		for i := range parsed {
			area, text := resolveTaskAreaAndTextWithMetadata(parsed[i].Text, parsed[i].Meta, noteArea)
			if strings.TrimSpace(text) == "" {
				continue
			}

			key := selectionKey(sourceID, text)
			if picks, ok := dashboardSelections[key]; ok {
				idx := selectionCursor[key]
				if idx < len(picks) {
					desiredDone := picks[idx]
					selectionCursor[key] = idx + 1
					if parsed[i].Done != desiredDone {
						lineIdx := parsed[i].Line - 1
						if lineIdx >= 0 && lineIdx < len(lines) {
							updatedLine := updateCheckboxInLine(lines[lineIdx], desiredDone)
							if updatedLine != lines[lineIdx] {
								lines[lineIdx] = updatedLine
								parsed[i].Done = desiredDone
								noteChanged = true
								stats.SourceWritebacks++
							}
						}
					}
				}
			}

			tasks = append(tasks, sourcedTask{
				Area:       area,
				Text:       text,
				Done:       parsed[i].Done,
				SourcePath: rel,
				SourceID:   sourceID,
				Order:      parsed[i].Order,
			})
		}

		if noteChanged {
			updated := strings.Join(lines, "\n")
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("write task source %s: %w", path, err)
			}
			stats.SourceFilesUpdated++
		}

		return nil
	})
	if err != nil {
		return stats, err
	}

	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].SourcePath != tasks[j].SourcePath {
			return tasks[i].SourcePath < tasks[j].SourcePath
		}
		return tasks[i].Order < tasks[j].Order
	})

	grouped := map[string][]sourcedTask{}
	for _, group := range focusGroups {
		grouped[group] = []sourcedTask{}
	}

	for _, t := range tasks {
		grouped[t.Area] = append(grouped[t.Area], t)
		stats.SyncedTasks++
		stats.TasksByArea[t.Area]++
	}

	if err := writeDashboardProjection(dashboardPath, grouped); err != nil {
		return stats, err
	}

	return stats, nil
}

func extractTodoTasks(content string) []parsedTask {
	lines := strings.Split(content, "\n")
	out := []parsedTask{}
	inTodo := false
	todoLevel := 0
	order := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		level, heading, ok := parseHeading(trimmed)
		if ok {
			if strings.EqualFold(heading, "Todo") {
				inTodo = true
				todoLevel = level
			} else if inTodo && level <= todoLevel {
				inTodo = false
			}
			continue
		}

		if !inTodo {
			continue
		}

		m := checkboxTaskRe.FindStringSubmatch(trimmed)
		if len(m) == 0 {
			continue
		}

		done := strings.EqualFold(m[1], "x")
		text := strings.TrimSpace(m[2])
		if text == "" {
			continue
		}

		taskIndent := lineIndent(line)
		j := i + 1
		metaParts := []string{}
		for ; j < len(lines); j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" {
				continue
			}
			if _, _, isHeading := parseHeading(nextTrimmed); isHeading {
				break
			}
			if lineIndent(lines[j]) <= taskIndent {
				break
			}
			metaLine := metadataBulletPrefixRe.ReplaceAllString(nextTrimmed, "")
			metaParts = append(metaParts, metaLine)
		}

		out = append(out, parsedTask{
			Text:  text,
			Done:  done,
			Order: order,
			Line:  i + 1,
			Meta:  strings.Join(metaParts, " "),
		})
		order++
		i = j - 1
	}

	return out
}

func resolveTaskAreaAndText(taskText, noteArea string) (string, string) {
	return resolveTaskAreaAndTextWithMetadata(taskText, "", noteArea)
}

func resolveTaskAreaAndTextWithMetadata(taskText, metadataText, noteArea string) (string, string) {
	inlineArea, cleaned, found, ok := extractInlineArea(taskText)
	if found && ok {
		return inlineArea, cleaned
	}
	if found && !ok {
		return noteArea, taskText
	}

	if area, ok := extractAreaFromMetadataLine(metadataText); ok {
		return area, taskText
	}

	return noteArea, taskText
}

func extractAreaFromMetadataLine(line string) (string, bool) {
	line = strings.TrimSpace(metadataBulletPrefixRe.ReplaceAllString(strings.TrimSpace(line), ""))
	if line == "" {
		return "", false
	}

	m := metadataAreaTokenRe.FindStringSubmatch(line)
	if len(m) < 2 {
		return "", false
	}

	area := resolveArea(strings.TrimSpace(m[1]), "")
	if area == "" {
		return "", false
	}
	return area, true
}

func extractInlineArea(taskText string) (string, string, bool, bool) {
	match := inlineAreaRe.FindStringSubmatchIndex(taskText)
	if len(match) < 4 {
		return "", taskText, false, false
	}

	raw := strings.TrimSpace(taskText[match[2]:match[3]])
	area := resolveArea(raw, "")
	if area == "" {
		return "", taskText, true, false
	}

	cleaned := strings.TrimSpace(taskText[:match[0]] + " " + taskText[match[1]:])
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if cleaned == "" {
		cleaned = taskText
	}

	return area, cleaned, true, true
}

func resolveArea(rawArea, fallback string) string {
	rawArea = strings.TrimSpace(rawArea)
	if rawArea == "" {
		return fallback
	}

	for _, group := range focusGroups {
		if strings.EqualFold(group, rawArea) {
			return group
		}
	}

	if strings.EqualFold(rawArea, "actions") {
		return "Action"
	}

	return fallback
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

func writeDashboardProjection(dashboardPath string, grouped map[string][]sourcedTask) error {
	prefix := defaultDashboardFrontmatter

	if b, err := os.ReadFile(dashboardPath); err == nil {
		if fm, ok := extractFrontmatterPrefix(string(b)); ok {
			prefix = fm
		}
	}

	if err := os.MkdirAll(filepath.Dir(dashboardPath), 0o755); err != nil {
		return fmt.Errorf("create dashboard directory: %w", err)
	}

	var b strings.Builder
	b.WriteString(prefix)
	if !strings.HasSuffix(prefix, "\n") {
		b.WriteString("\n")
	}

	for _, group := range focusGroups {
		b.WriteString("# " + group + "\n")
		for _, task := range grouped[group] {
			box := "[ ]"
			if task.Done {
				box = "[x]"
			}
			text := strings.TrimSpace(task.Text)
			if task.SourceID != "" {
				text = text + " [[" + task.SourceID + "]]"
			}
			b.WriteString("- " + box + " " + text + "\n")
		}
	}

	if err := os.WriteFile(dashboardPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write dashboard: %w", err)
	}
	return nil
}

func extractFrontmatterPrefix(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[:i+1], "\n") + "\n", true
		}
	}
	return "", false
}

func readDashboardSelections(dashboardPath string) (map[string][]bool, error) {
	out := map[string][]bool{}
	b, err := os.ReadFile(dashboardPath)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, fmt.Errorf("read dashboard for writeback: %w", err)
	}

	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		m := checkboxTaskRe.FindStringSubmatch(t)
		if len(m) == 0 {
			continue
		}

		taskText := strings.TrimSpace(m[2])
		text, sourceID, ok := splitDashboardTaskSource(taskText)
		if !ok {
			continue
		}

		key := selectionKey(sourceID, text)
		out[key] = append(out[key], strings.EqualFold(m[1], "x"))
	}

	return out, nil
}

func splitDashboardTaskSource(taskText string) (string, string, bool) {
	loc := dashboardSourceSuffixRe.FindStringSubmatchIndex(taskText)
	if len(loc) < 4 {
		return "", "", false
	}
	sourceID := strings.TrimSpace(taskText[loc[2]:loc[3]])
	text := strings.TrimSpace(taskText[:loc[0]])
	if sourceID == "" || text == "" {
		return "", "", false
	}
	return text, sourceID, true
}

func selectionKey(sourceID, taskText string) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(taskText)), " ")
	return strings.TrimSpace(sourceID) + "\x1f" + normalized
}

func updateCheckboxInLine(line string, done bool) string {
	newBox := "[ ]"
	if done {
		newBox = "[x]"
	}
	loc := checkboxStateRe.FindStringIndex(line)
	if loc == nil {
		return line
	}
	return line[:loc[0]] + newBox + line[loc[1]:]
}

const defaultDashboardFrontmatter = "---\nid: \"dashboard\"\ntags: []\n---\n"
