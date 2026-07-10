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
	"time"
)

type ArchiveStats struct {
	ScannedMarkdownNotes int
	ActiveTaskIndexNotes int
	ArchivedTasks        int
	ArchivedByArea       map[string]int
	MonthFilesUpdated    int
	SourceFilesUpdated   int
}

type archivedTask struct {
	Area     string
	Text     string
	SourceID string
	DoneDate time.Time
	Meta     string
}

type sourceArchiveEdit struct {
	Path    string
	Content string
}

type todoTaskBlock struct {
	StartLine int
	EndLine   int
	Done      bool
	Area      string
	CleanText string
	DoneDate  time.Time
	Meta      string
}

var doneTokenRe = regexp.MustCompile(`(?i)\bdone:\s*(\d{4}-\d{2}-\d{2})\b`)

func ArchiveCompletedToLifeLog(notesDir string) (ArchiveStats, error) {
	return archiveTasksToLifeLog(notesDir, func(_ string, block todoTaskBlock, _ int) bool {
		return block.Done
	})
}

func ArchiveSelectedToLifeLog(notesDir string, selectionKeys []string) (ArchiveStats, error) {
	selected := map[string]struct{}{}
	for _, raw := range selectionKeys {
		if key := normalizeArchiveSelectionKey(raw); key != "" {
			selected[key] = struct{}{}
		}
	}
	if len(selected) == 0 {
		return ArchiveStats{ArchivedByArea: map[string]int{}}, nil
	}

	return archiveTasksToLifeLog(notesDir, func(sourceID string, block todoTaskBlock, occurrence int) bool {
		key := archiveSelectionKey(sourceID, block.CleanText, occurrence)
		_, ok := selected[key]
		return ok
	})
}

func archiveTasksToLifeLog(notesDir string, shouldArchive func(sourceID string, block todoTaskBlock, occurrence int) bool) (ArchiveStats, error) {
	stats := ArchiveStats{ArchivedByArea: map[string]int{}}
	root := filepath.Clean(notesDir)
	lifeLogRoot := resolveLifeLogRoot(root)

	monthBuckets := map[string]map[string]map[string][]archivedTask{}
	sourceEdits := []sourceArchiveEdit{}
	now := time.Now()

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

		stats.ScannedMarkdownNotes++

		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		content := string(b)

		meta, ok := readFrontmatterMap(content)
		if !ok {
			return nil
		}
		if !hasDomain(meta, "task-index") {
			return nil
		}
		if !readBool(meta, "task_active") {
			return nil
		}

		stats.ActiveTaskIndexNotes++
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

		lines := strings.Split(content, "\n")
		blocks := extractTodoTaskBlocks(lines, noteArea)
		if len(blocks) == 0 {
			return nil
		}

		occurrenceByText := map[string]int{}
		removeRanges := [][2]int{}
		for _, block := range blocks {
			normalizedText := normalizeArchiveTaskText(block.CleanText)
			occurrenceByText[normalizedText]++
			occurrence := occurrenceByText[normalizedText]

			if !shouldArchive(sourceID, block, occurrence) {
				continue
			}

			doneDate := block.DoneDate
			if doneDate.IsZero() {
				doneDate = now
			}
			doneDate = startOfDay(doneDate)

			yearDir, err := ensureYearFolder(lifeLogRoot, doneDate.Year())
			if err != nil {
				return err
			}
			monthPath := filepath.Join(yearDir, fmt.Sprintf("%04d-%02d.md", doneDate.Year(), int(doneDate.Month())))
			weekKey := weekStart(doneDate).Format("2006-01-02")

			if _, ok := monthBuckets[monthPath]; !ok {
				monthBuckets[monthPath] = map[string]map[string][]archivedTask{}
			}
			if _, ok := monthBuckets[monthPath][weekKey]; !ok {
				monthBuckets[monthPath][weekKey] = map[string][]archivedTask{}
			}
			monthBuckets[monthPath][weekKey][block.Area] = append(monthBuckets[monthPath][weekKey][block.Area], archivedTask{
				Area:     block.Area,
				Text:     strings.TrimSpace(block.CleanText),
				SourceID: sourceID,
				DoneDate: doneDate,
				Meta:     strings.TrimSpace(block.Meta),
			})

			removeRanges = append(removeRanges, [2]int{block.StartLine, block.EndLine})
			stats.ArchivedTasks++
			stats.ArchivedByArea[block.Area]++
		}

		if len(removeRanges) == 0 {
			return nil
		}

		updated := removeLineRanges(lines, removeRanges)
		updatedContent := strings.Join(updated, "\n")
		if updatedContent != content {
			sourceEdits = append(sourceEdits, sourceArchiveEdit{Path: path, Content: updatedContent})
		}

		return nil
	})
	if err != nil {
		return stats, err
	}

	if err := writeArchiveMonthFiles(monthBuckets); err != nil {
		return stats, err
	}
	stats.MonthFilesUpdated = len(monthBuckets)

	for _, edit := range sourceEdits {
		if err := os.WriteFile(edit.Path, []byte(edit.Content), 0o644); err != nil {
			return stats, fmt.Errorf("write task source %s: %w", edit.Path, err)
		}
		stats.SourceFilesUpdated++
	}

	return stats, nil
}

func resolveLifeLogRoot(notesRoot string) string {
	override := strings.TrimSpace(os.Getenv("MW_LIFE_LOG_DIR"))
	if override != "" {
		if filepath.IsAbs(override) {
			return filepath.Clean(override)
		}
		return filepath.Join(notesRoot, filepath.FromSlash(override))
	}

	return filepath.Join(notesRoot, "introspection", "life-log")
}

func extractTodoTaskBlocks(lines []string, noteArea string) []todoTaskBlock {
	blocks := []todoTaskBlock{}
	inTodo := false
	todoLevel := 0

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])

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

		m := checkboxTaskRe.FindStringSubmatch(trimmed)
		if len(m) == 0 {
			continue
		}

		taskIndent := lineIndent(lines[i])
		j := i + 1
		metaParts := []string{}
		childArea := ""
		for ; j < len(lines); j++ {
			nextTrimmed := strings.TrimSpace(lines[j])
			if nextTrimmed == "" {
				continue
			}
			if _, _, isHeading := parseHeading(nextTrimmed); isHeading {
				break
			}
			nextIndent := lineIndent(lines[j])
			if nextIndent <= taskIndent {
				break
			}
			if childArea == "" {
				if area, ok := extractAreaFromMetadataLine(nextTrimmed); ok {
					childArea = area
				}
			}
			if tokenLine := metadataTokensFromLine(nextTrimmed); tokenLine != "" {
				metaParts = append(metaParts, tokenLine)
			}
		}

		done := strings.EqualFold(m[1], "x")
		rawTaskText := strings.TrimSpace(m[2])
		area, cleaned := resolveTaskAreaAndTextWithMetadata(rawTaskText, childArea, noteArea)
		if strings.TrimSpace(cleaned) == "" {
			cleaned = strings.TrimSpace(rawTaskText)
		}

		meta := strings.Join(metaParts, " ")
		combined := strings.TrimSpace(rawTaskText + " " + meta)
		doneDate := parseDoneDate(combined)

		blocks = append(blocks, todoTaskBlock{
			StartLine: i,
			EndLine:   j,
			Done:      done,
			Area:      area,
			CleanText: cleaned,
			DoneDate:  doneDate,
			Meta:      meta,
		})

		i = j - 1
	}

	return blocks
}

func parseDoneDate(text string) time.Time {
	m := doneTokenRe.FindStringSubmatch(text)
	if len(m) < 2 {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(m[1]), time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

func archiveSelectionKey(sourceID, taskText string, occurrence int) string {
	return strings.TrimSpace(sourceID) + "\x1f" + normalizeArchiveTaskText(taskText) + "\x1f" + strconv.Itoa(occurrence)
}

func normalizeArchiveSelectionKey(raw string) string {
	parts := strings.Split(raw, "\x1f")
	if len(parts) != 3 {
		return ""
	}
	sourceID := strings.TrimSpace(parts[0])
	taskText := normalizeArchiveTaskText(parts[1])
	occurrence, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil || occurrence <= 0 {
		return ""
	}
	if sourceID == "" || taskText == "" {
		return ""
	}
	return archiveSelectionKey(sourceID, taskText, occurrence)
}

func normalizeArchiveTaskText(taskText string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(taskText)), " ")
}

func removeLineRanges(lines []string, ranges [][2]int) []string {
	if len(ranges) == 0 {
		return lines
	}

	skip := make([]bool, len(lines))
	for _, r := range ranges {
		start, end := r[0], r[1]
		if start < 0 {
			start = 0
		}
		if end > len(lines) {
			end = len(lines)
		}
		for i := start; i < end; i++ {
			skip[i] = true
		}
	}

	out := make([]string, 0, len(lines))
	for i, line := range lines {
		if skip[i] {
			continue
		}
		out = append(out, line)
	}

	compact := make([]string, 0, len(out))
	emptyRun := 0
	for _, line := range out {
		if strings.TrimSpace(line) == "" {
			emptyRun++
			if emptyRun > 2 {
				continue
			}
		} else {
			emptyRun = 0
		}
		compact = append(compact, line)
	}
	return compact
}

func ensureYearFolder(lifeLogRoot string, year int) (string, error) {
	if err := os.MkdirAll(lifeLogRoot, 0o755); err != nil {
		return "", fmt.Errorf("create life-log root %s: %w", lifeLogRoot, err)
	}

	entries, err := os.ReadDir(lifeLogRoot)
	if err != nil {
		return "", fmt.Errorf("read life-log root %s: %w", lifeLogRoot, err)
	}

	prefix := fmt.Sprintf("y%d", year)
	candidates := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(e.Name()))
		if name == strings.ToLower(prefix) || strings.HasPrefix(name, strings.ToLower(prefix)+"-") {
			candidates = append(candidates, filepath.Join(lifeLogRoot, e.Name()))
		}
	}

	if len(candidates) > 0 {
		sort.Strings(candidates)
		return candidates[0], nil
	}

	yearDir := filepath.Join(lifeLogRoot, fmt.Sprintf("y%d-archive", year))
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		return "", fmt.Errorf("create life-log year folder %s: %w", yearDir, err)
	}
	return yearDir, nil
}

func writeArchiveMonthFiles(monthBuckets map[string]map[string]map[string][]archivedTask) error {
	monthPaths := make([]string, 0, len(monthBuckets))
	for monthPath := range monthBuckets {
		monthPaths = append(monthPaths, monthPath)
	}
	sort.Strings(monthPaths)

	for _, monthPath := range monthPaths {
		if err := os.MkdirAll(filepath.Dir(monthPath), 0o755); err != nil {
			return fmt.Errorf("create month parent directory: %w", err)
		}

		existing := ""
		if b, err := os.ReadFile(monthPath); err == nil {
			existing = string(b)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read month file %s: %w", monthPath, err)
		}

		updated := appendArchiveContent(existing, monthPath, monthBuckets[monthPath])
		if updated == existing {
			continue
		}

		if err := os.WriteFile(monthPath, []byte(updated), 0o644); err != nil {
			return fmt.Errorf("write month file %s: %w", monthPath, err)
		}
	}

	return nil
}

func appendArchiveContent(existing, monthPath string, weeks map[string]map[string][]archivedTask) string {
	var b strings.Builder
	if strings.TrimSpace(existing) == "" {
		monthName := strings.TrimSuffix(filepath.Base(monthPath), filepath.Ext(monthPath))
		b.WriteString("# " + monthName + "\n\n")
	} else {
		b.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			b.WriteString("\n")
		}
		if !strings.HasSuffix(existing, "\n\n") {
			b.WriteString("\n")
		}
	}

	weekKeys := make([]string, 0, len(weeks))
	for week := range weeks {
		weekKeys = append(weekKeys, week)
	}
	sort.Strings(weekKeys)

	for _, week := range weekKeys {
		b.WriteString("## Week of " + week + "\n")

		areas := weeks[week]
		areaKeys := make([]string, 0, len(areas))
		for area := range areas {
			areaKeys = append(areaKeys, area)
		}
		sort.Strings(areaKeys)

		for _, area := range areaKeys {
			b.WriteString("### " + area + "\n")
			for _, t := range areas[area] {
				doneDate := startOfDay(t.DoneDate).Format("2006-01-02")
				signature := fmt.Sprintf("- [x] %s [[%s]] (done:%s)", strings.TrimSpace(t.Text), strings.TrimSpace(t.SourceID), doneDate)
				if strings.Contains(existing, signature) || strings.Contains(b.String(), signature) {
					continue
				}

				b.WriteString(signature + "\n")
				if strings.TrimSpace(t.Meta) != "" {
					b.WriteString("  - meta: " + strings.TrimSpace(t.Meta) + "\n")
				}
				b.WriteString("  - reflection:\n")
			}
		}
		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n") + "\n"
}

func weekStart(t time.Time) time.Time {
	t = startOfDay(t)
	offset := (int(t.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
	return t.AddDate(0, 0, -offset)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
