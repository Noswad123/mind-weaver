package todos

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
)

type todoWeightDefaults struct {
	Priority string
	Energy   string
}

var (
	metadataBulletPrefixRe = regexp.MustCompile(`^[*+-]\s+`)

	priorityMetaRe       = regexp.MustCompile(`(?i)\bpriority:\s*(p?[1-5])\b`)
	priorityInlineMetaRe = regexp.MustCompile(`(?i)\bp:?([1-5])\b`)

	energyMetaRe       = regexp.MustCompile(`(?i)\benergy:\s*([a-zA-Z-]+)\b`)
	energyInlineMetaRe = regexp.MustCompile(`(?i)\be:(xsm|xs|x-small|small|s|medium|m|large|l|xl|x-large)\b`)

	weightMetaRe = regexp.MustCompile(`(?i)\b(?:w|weight):\s*([0-9]+(?:\.[0-9]+)?)\b`)
	dueMetaRe    = regexp.MustCompile(`(?i)\bdue:\s*(\d{4}-\d{2}-\d{2})\b`)
	startMetaRe  = regexp.MustCompile(`(?i)\bstart:\s*(\d{4}-\d{2}-\d{2})\b`)
	estMetaRe    = regexp.MustCompile(`(?i)\b(?:est|estimate):\s*([0-9]+)\b`)
)

func loadTaskIndexWeightContext(notesDir string) (map[string]todoWeightDefaults, map[string][]string, error) {
	defaults := map[string]todoWeightDefaults{}
	metadataByTaskKey := map[string][]string{}
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

		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		rel = filepath.ToSlash(filepath.Clean(rel))

		sourceID := strings.TrimSpace(readString(meta, "id"))
		if sourceID == "" {
			sourceID = rel
		}
		noteArea := resolveArea(readString(meta, "task_area"), "Action")

		defaults[sourceID] = todoWeightDefaults{
			Priority: strings.TrimSpace(readString(meta, "task_default_priority")),
			Energy:   strings.TrimSpace(readString(meta, "task_default_energy")),
		}

		for key, values := range extractTaskMetadataByKey(content, sourceID, noteArea) {
			metadataByTaskKey[key] = append(metadataByTaskKey[key], values...)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return defaults, metadataByTaskKey, nil
}

func extractTaskMetadataByKey(content, sourceID, noteArea string) map[string][]string {
	out := map[string][]string{}
	lines := strings.Split(content, "\n")
	inTodo := false
	todoLevel := 0

	for i := 0; i < len(lines); i++ {
		line := lines[i]
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

		m := checkboxTaskRe.FindStringSubmatch(trimmed)
		if len(m) == 0 {
			continue
		}

		taskText := strings.TrimSpace(m[2])
		_, cleanedText := resolveTaskAreaAndText(taskText, noteArea)
		key := selectionKey(sourceID, cleanedText)
		taskIndent := lineIndent(line)

		metaTokens := []string{}
		for j := i + 1; j < len(lines); j++ {
			nextLine := lines[j]
			nextTrimmed := strings.TrimSpace(nextLine)
			if nextTrimmed == "" {
				continue
			}
			if _, _, isHeading := parseHeading(nextTrimmed); isHeading {
				break
			}

			nextIndent := lineIndent(nextLine)
			if nextIndent <= taskIndent {
				break
			}

			if tokenLine := metadataTokensFromLine(nextTrimmed); tokenLine != "" {
				metaTokens = append(metaTokens, tokenLine)
			}
		}

		out[key] = append(out[key], strings.Join(metaTokens, " "))
	}

	return out
}

func metadataTokensFromLine(line string) string {
	line = strings.TrimSpace(metadataBulletPrefixRe.ReplaceAllString(strings.TrimSpace(line), ""))
	if line == "" {
		return ""
	}

	tokens := []string{}

	if m := priorityMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		if p, ok := normalizePriorityToken(m[1]); ok {
			tokens = append(tokens, p)
		}
	} else if m := priorityInlineMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		tokens = append(tokens, "p"+m[1])
	}

	if m := energyMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		if e, ok := normalizeEnergyToken(m[1]); ok {
			tokens = append(tokens, "e:"+e)
		}
	} else if m := energyInlineMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		if e, ok := normalizeEnergyToken(m[1]); ok {
			tokens = append(tokens, "e:"+e)
		}
	}

	if m := weightMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		tokens = append(tokens, "w:"+strings.TrimSpace(m[1]))
	}
	if m := dueMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		tokens = append(tokens, "due:"+strings.TrimSpace(m[1]))
	}
	if m := startMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		tokens = append(tokens, "start:"+strings.TrimSpace(m[1]))
	}
	if m := estMetaRe.FindStringSubmatch(line); len(m) >= 2 {
		tokens = append(tokens, "est:"+strings.TrimSpace(m[1]))
	}

	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, " ")
}

func buildTodoMetadata(taskText, metadataText, area, todoSection string, done bool, defaultPriority, defaultEnergy string) TodoMetadata {
	raw := strings.TrimSpace(metadataText)
	combined := strings.TrimSpace(taskText + " " + raw)
	status := normalizeTodoWorkflowSection(todoSection)
	if done {
		status = "Done"
	}

	priority := extractPriority(combined)
	if priority == "" {
		priority = normalizePriorityOrDefault(defaultPriority, "p3")
	}
	energy := extractEnergy(combined)
	if energy == "" {
		energy = normalizeEnergyOrDefault(defaultEnergy, "medium")
	}

	meta := TodoMetadata{
		Status:          status,
		TodoSection:     normalizeTodoWorkflowSection(todoSection),
		Area:            strings.TrimSpace(area),
		Priority:        priority,
		Energy:          energy,
		WeightOverride:  extractFirst(weightMetaRe, combined),
		Due:             extractFirst(dueMetaRe, combined),
		Start:           extractFirst(startMetaRe, combined),
		Estimate:        extractFirst(estMetaRe, combined),
		Raw:             raw,
		DefaultPriority: normalizePriorityOrDefault(defaultPriority, "p3"),
		DefaultEnergy:   normalizeEnergyOrDefault(defaultEnergy, "medium"),
	}
	meta.EffectiveWeight = parser.DeriveTodoWeightWithDefaults(combined, meta.DefaultPriority, meta.DefaultEnergy)
	return meta
}

func buildUpdatedMetadataLine(existing TodoMetadata, params TodoUpdateParams) string {
	if params.Metadata != nil {
		return strings.TrimSpace(*params.Metadata)
	}

	meta := explicitTodoMetadata(existing)
	for _, raw := range params.Clear {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "area":
			meta.Area = ""
		case "priority", "p":
			meta.Priority = ""
		case "energy", "e":
			meta.Energy = ""
		case "weight", "w", "weightoverride":
			meta.WeightOverride = ""
		case "due":
			meta.Due = ""
		case "start":
			meta.Start = ""
		case "estimate", "est":
			meta.Estimate = ""
		}
	}

	if params.Area != nil {
		area := strings.TrimSpace(*params.Area)
		if area == "" {
			meta.Area = ""
		} else {
			meta.Area = resolveArea(area, area)
		}
	}
	if params.Priority != nil {
		meta.Priority = normalizePriorityOrDefault(*params.Priority, "")
	}
	if params.Energy != nil {
		meta.Energy = normalizeEnergyOrDefault(*params.Energy, "")
	}
	if params.Weight != nil {
		meta.WeightOverride = strings.TrimSpace(*params.Weight)
	}
	if params.Due != nil {
		meta.Due = strings.TrimSpace(*params.Due)
	}
	if params.Start != nil {
		meta.Start = strings.TrimSpace(*params.Start)
	}
	if params.Estimate != nil {
		meta.Estimate = strings.TrimSpace(*params.Estimate)
	}

	parts := []string{}
	if strings.TrimSpace(meta.Area) != "" {
		parts = append(parts, "area: "+strings.TrimSpace(meta.Area))
	}
	if strings.TrimSpace(meta.Priority) != "" {
		parts = append(parts, normalizePriorityOrDefault(meta.Priority, ""))
	}
	if strings.TrimSpace(meta.Energy) != "" {
		parts = append(parts, "e:"+shortEnergyToken(meta.Energy))
	}
	if strings.TrimSpace(meta.WeightOverride) != "" {
		parts = append(parts, "w:"+strings.TrimSpace(meta.WeightOverride))
	}
	if strings.TrimSpace(meta.Due) != "" {
		parts = append(parts, "due:"+strings.TrimSpace(meta.Due))
	}
	if strings.TrimSpace(meta.Start) != "" {
		parts = append(parts, "start:"+strings.TrimSpace(meta.Start))
	}
	if strings.TrimSpace(meta.Estimate) != "" {
		parts = append(parts, "est:"+strings.TrimSpace(meta.Estimate))
	}
	return strings.Join(parts, " ")
}

func explicitTodoMetadata(existing TodoMetadata) TodoMetadata {
	raw := strings.TrimSpace(existing.Raw)
	meta := TodoMetadata{Raw: raw}
	if area, ok := extractAreaFromMetadataLine(raw); ok {
		meta.Area = area
	}
	meta.Priority = extractPriority(raw)
	meta.Energy = extractEnergy(raw)
	meta.WeightOverride = extractFirst(weightMetaRe, raw)
	meta.Due = extractFirst(dueMetaRe, raw)
	meta.Start = extractFirst(startMetaRe, raw)
	meta.Estimate = extractFirst(estMetaRe, raw)
	return meta
}

func isTaskMetadataLine(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	stripped := strings.TrimSpace(metadataBulletPrefixRe.ReplaceAllString(trimmed, ""))
	if stripped == "" {
		return false
	}
	return metadataAreaTokenRe.MatchString(stripped) || metadataTokensFromLine(stripped) != ""
}

func extractPriority(text string) string {
	if m := priorityMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		return normalizePriorityOrDefault(m[1], "")
	}
	if m := priorityInlineMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		return "p" + strings.TrimSpace(m[1])
	}
	return ""
}

func extractEnergy(text string) string {
	if m := energyMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		return normalizeEnergyOrDefault(m[1], "")
	}
	if m := energyInlineMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		return normalizeEnergyOrDefault(m[1], "")
	}
	return ""
}

func extractFirst(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func normalizePriorityOrDefault(raw, fallback string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.TrimPrefix(v, "p")
	if v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 5 {
			return "p" + strconv.Itoa(n)
		}
	}
	if fallback != "" {
		return fallback
	}
	return ""
}

func normalizeEnergyOrDefault(raw, fallback string) string {
	if e, ok := normalizeEnergyToken(raw); ok {
		return e
	}
	if fallback != "" {
		return fallback
	}
	return ""
}

func shortEnergyToken(raw string) string {
	energy := normalizeEnergyOrDefault(raw, raw)
	switch energy {
	case "x-small":
		return "xsm"
	case "small":
		return "s"
	case "medium":
		return "m"
	case "large":
		return "l"
	case "x-large":
		return "xl"
	default:
		return strings.TrimSpace(raw)
	}
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

func applyTodoWeights(todoMap map[string][]parser.Todo, defaultsBySource map[string]todoWeightDefaults, metadataByTaskKey map[string][]string) {
	metadataCursor := map[string]int{}

	for group, todos := range todoMap {
		for i := range todos {
			text := todos[i].Text
			defaults := todoWeightDefaults{Priority: "p3", Energy: "medium"}
			metaText := ""

			if taskText, sourceID, ok := splitDashboardTaskSource(text); ok {
				text = taskText
				if d, found := defaultsBySource[sourceID]; found {
					if d.Priority != "" {
						defaults.Priority = d.Priority
					}
					if d.Energy != "" {
						defaults.Energy = d.Energy
					}
				}

				key := selectionKey(sourceID, text)
				if items := metadataByTaskKey[key]; len(items) > 0 {
					idx := metadataCursor[key]
					if idx < len(items) {
						metaText = items[idx]
					}
					metadataCursor[key] = idx + 1
				}
			}

			combined := text
			if strings.TrimSpace(metaText) != "" {
				combined = combined + " " + metaText
			}

			weight := parser.DeriveTodoWeightWithDefaults(combined, defaults.Priority, defaults.Energy)
			todos[i].Weight = weight
			todos[i].MetaSummary = buildTodoMetaSummary(combined, defaults, weight)
		}
		todoMap[group] = todos
	}
}

func buildTodoMetaSummary(text string, defaults todoWeightDefaults, weight float64) string {
	priority := effectivePriority(text, defaults.Priority)
	energy := effectiveEnergy(text, defaults.Energy)
	parts := []string{priority, "e:" + energy}

	if due := firstMatch(dueMetaRe, text); due != "" {
		parts = append(parts, "due:"+due)
	}
	if start := firstMatch(startMetaRe, text); start != "" {
		parts = append(parts, "start:"+start)
	}

	parts = append(parts, fmt.Sprintf("w:%.2f", weight))
	return strings.Join(parts, "  ")
}

func effectivePriority(text, defaultPriority string) string {
	if m := priorityMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		if p, ok := normalizePriorityToken(m[1]); ok {
			return p
		}
	}
	if m := priorityInlineMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		return "p" + m[1]
	}
	if p, ok := normalizePriorityToken(defaultPriority); ok {
		return p
	}
	return "p3"
}

func effectiveEnergy(text, defaultEnergy string) string {
	if m := energyMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		if e, ok := normalizeEnergyToken(m[1]); ok {
			return e
		}
	}
	if m := energyInlineMetaRe.FindStringSubmatch(text); len(m) >= 2 {
		if e, ok := normalizeEnergyToken(m[1]); ok {
			return e
		}
	}
	if e, ok := normalizeEnergyToken(defaultEnergy); ok {
		return e
	}
	return "m"
}

func normalizePriorityToken(raw string) (string, bool) {
	v := strings.TrimSpace(strings.ToLower(raw))
	v = strings.TrimPrefix(v, "p")
	if v == "" {
		return "", false
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 5 {
		return "", false
	}
	return "p" + strconv.Itoa(n), true
}

func normalizeEnergyToken(raw string) (string, bool) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "xsm", "xs", "x-small":
		return "xsm", true
	case "small", "s":
		return "s", true
	case "m", "medium":
		return "m", true
	case "large", "l":
		return "l", true
	case "xl", "x-large":
		return "xl", true
	default:
		return "", false
	}
}

func firstMatch(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
