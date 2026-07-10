package formatter

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/markdown"
)

type DirSnapshot struct {
	DirPath     string
	HubPath     string
	HubID       string
	ChildHubIDs map[string]bool // immediate child hub ids (for stale checks)
	RelNoteIDs  map[string]bool // immediate .md files (basename) for stale checks
}

const topicsHeader = "## Topics"

var (
	canonicalHubSections = []string{"Todo", "Topics", "Research", "Resources"}
	wikiBulletRe         = regexp.MustCompile(`^\s*[-*]\s+\[\[([^\]\r\n]+)\]\]\s*$`)
	relBulletRe          = regexp.MustCompile(`^\s*[-*]\s+\{\:([^}:]+)\:\}\s*$`)
)

func planUpdate(originalContent string, snap DirSnapshot) (updated string, didChange bool, issues []string, err error) {
	issues = []string{}

	content, metaIssues := ensureHubFrontmatter(originalContent, snap)
	issues = append(issues, metaIssues...)

	lines := strings.Split(content, "\n")
	lines = normalizeHubSectionHeadings(lines)
	lines, _ = normalizeMarkdownHeadingLines(lines, hubDisplayTitle(snap))

	before := strings.Join(lines, "\n")
	lines = EnsureHeaders(lines, requiredHeaders)
	if strings.Join(lines, "\n") != before {
		issues = append(issues, "missing required headers")
	}

	lines = reconcileTopicsSection(lines, snap)

	out := strings.Join(lines, "\n")
	out = strings.TrimSpace(out) + "\n"

	return out, out != strings.TrimSpace(originalContent)+"\n", issues, nil
}

func ensureHubFrontmatter(content string, snap DirSnapshot) (string, []string) {
	issues := []string{}

	if !markdown.HasMetaBlock(content) {
		issues = append(issues, "missing frontmatter")
		genID := generateHubID(snap)
		content = fmt.Sprintf("---\nid: %q\ntags: []\n---\n\n", genID) + content
		return content, issues
	}

	id, _ := markdown.ReadMetaIDFromContent(content)
	if strings.TrimSpace(id) == "" {
		issues = append(issues, "missing id in frontmatter")
		genID := generateHubID(snap)
		content = markdown.EnsureMetaID(content, genID)
	}

	return content, issues
}

func normalizeHubSectionHeadings(lines []string) []string {
	out := make([]string, len(lines))
	copy(out, lines)
	inFence := false

	for i, line := range out {
		trimmed := strings.TrimSpace(line)
		if isFenceDelimiter(trimmed) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}

		_, heading, ok := parseHashHeading(trimmed)
		if !ok {
			continue
		}

		canonical, isSection := canonicalHubSectionName(heading)
		if !isSection {
			continue
		}
		out[i] = "## " + canonical
	}

	return out
}

func reconcileTopicsSection(lines []string, snap DirSnapshot) []string {
	start, end, ok := sectionBounds(lines, topicsHeader)
	if !ok {
		return lines
	}

	allowedIDs := buildAllowedTopicIDs(snap)
	allowedSet := map[string]bool{}
	for _, id := range allowedIDs {
		allowedSet[id] = true
	}

	body := lines[start+1 : end]
	seen := map[string]bool{}
	cleaned := make([]string, 0, len(body)+len(allowedIDs))

	for _, line := range body {
		id, ok := topicLinkID(line)
		if !ok {
			continue
		}
		if !allowedSet[id] || seen[id] {
			continue
		}
		cleaned = append(cleaned, line)
		seen[id] = true
	}

	for _, id := range allowedIDs {
		if seen[id] {
			continue
		}
		cleaned = append(cleaned, fmt.Sprintf("- [[%s]]", id))
	}

	return replaceSectionBody(lines, start, end, cleaned)
}

func buildAllowedTopicIDs(snap DirSnapshot) []string {
	childIDs := make([]string, 0, len(snap.ChildHubIDs))
	for id := range snap.ChildHubIDs {
		childIDs = append(childIDs, id)
	}
	sort.Strings(childIDs)

	noteIDs := make([]string, 0, len(snap.RelNoteIDs))
	for id := range snap.RelNoteIDs {
		noteIDs = append(noteIDs, id)
	}
	sort.Strings(noteIDs)

	out := make([]string, 0, len(childIDs)+len(noteIDs))
	out = append(out, childIDs...)
	out = append(out, noteIDs...)
	return out
}

func topicLinkID(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if m := wikiBulletRe.FindStringSubmatch(t); m != nil {
		id := strings.TrimSpace(m[1])
		return id, id != ""
	}
	if m := relBulletRe.FindStringSubmatch(t); m != nil {
		id := strings.TrimSpace(m[1])
		return id, id != ""
	}
	return "", false
}

func sectionBounds(lines []string, section string) (start int, end int, ok bool) {
	start = -1
	for i, line := range lines {
		if strings.TrimSpace(line) == section {
			start = i
			break
		}
	}
	if start == -1 {
		return 0, 0, false
	}

	end = len(lines)
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "#") {
			end = i
			break
		}
	}

	return start, end, true
}

func replaceSectionBody(lines []string, start, end int, entries []string) []string {
	prefix := append([]string{}, lines[:start+1]...)
	suffix := append([]string{}, lines[end:]...)
	for len(suffix) > 0 && strings.TrimSpace(suffix[0]) == "" {
		suffix = suffix[1:]
	}

	out := make([]string, 0, len(prefix)+len(entries)+len(suffix)+1)
	out = append(out, prefix...)
	out = append(out, entries...)
	if len(suffix) > 0 {
		out = append(out, "")
	}
	out = append(out, suffix...)

	return out
}

func parseHashHeading(line string) (int, string, bool) {
	if line == "" || line[0] != '#' {
		return 0, "", false
	}

	i := 0
	for i < len(line) && line[i] == '#' {
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

func canonicalHubSectionName(name string) (string, bool) {
	for _, section := range canonicalHubSections {
		if strings.EqualFold(strings.TrimSpace(name), section) {
			return section, true
		}
	}
	return "", false
}

func hubDisplayTitle(snap DirSnapshot) string {
	title := strings.TrimSpace(filepath.Base(snap.DirPath))
	title = strings.ReplaceAll(title, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return "Hub"
	}
	return title
}

func generateHubID(snap DirSnapshot) string {
	return filepath.Base(snap.DirPath)
}
