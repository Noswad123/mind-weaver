package parser

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/note"
)

type MarkdownParser struct{}

func (MarkdownParser) Parse(content string, filePath string) (ParsedNote, error) {

	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	metadata := ExtractMetadata(content)
	metadataTags := metadata.Tags
	metadataDomains := metadata.Domains

	tags := make([]string, 0)
	if len(metadataTags) > 0 {
		tags = append(tags, metadataTags...)
	}

	domains := make([]string, 0)
	if len(metadataDomains) > 0 {
		domains = append(domains, metadataDomains...)
	}

	tagPattern := regexp.MustCompile(`:([a-zA-Z0-9_-]+):`)
	matches := tagPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		tags = append(tags, m[1])
	}

	parsedLinks := []note.Link{}

	// TODO:: Find Correct pattern for markdown links
	// Pattern A: {:label:}[link], Pattern B: [link]{:label:}
	internal := regexp.MustCompile(`\{\:([^\}]+)\:\}\[([^\]]+)]|\[([^\]]+)]\{\:([^\}]+)\:\}`)
	external := regexp.MustCompile(`\{(https?:\/\/[^\}]+)}\[([^\]]+)]|\[([^\]]+)]\{(https?:\/\/[^\}]+)}`)

	for _, m := range internal.FindAllStringSubmatch(content, -1) {
		rawPath := m[1]
		if rawPath == "" {
			rawPath = m[4]
		}
		label := m[2]
		if label == "" {
			label = m[3]
		}
		resolved := ResolveInternalLink(rawPath)
		parsedLinks = append(parsedLinks, note.Link{
			Type:         "internal",
			Target:       rawPath,
			Label:        label,
			ResolvedPath: resolved,
		})
	}

	for _, m := range external.FindAllStringSubmatch(content, -1) {
		url := m[1]
		if url == "" {
			url = m[4]
		}
		label := m[2]
		if label == "" {
			label = m[3]
		}
		parsedLinks = append(parsedLinks, note.Link{
			Type:         "external",
			Target:       url,
			Label:        label,
			ResolvedPath: "",
		})
	}
	return ParsedNote{
		Title:   title,
		Domains: domains,
		Tags:    tags,
		Meta:    metadata.Raw,
		Todos:   []Todo{},
		Links:   parsedLinks,
		Content: content,
	}, nil
}
func (m MarkdownParser) ParseDashboard(content string, targetGroups []string) map[string][]Todo {
	lines := strings.Split(content, "\n")
	results := make(map[string][]Todo)
	var currentGroup string

	inFrontmatter := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "---") {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "# ") {
			cleanHeading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))

			currentGroup = ""
			for _, group := range targetGroups {
				if strings.EqualFold(cleanHeading, group) {
					currentGroup = group
					break
				}
			}
			continue
		}

		if currentGroup != "" {
			isChecked := strings.Contains(trimmed, "[x]") || strings.Contains(trimmed, "[X]")
			isUnchecked := strings.Contains(trimmed, "[ ]")

			if isChecked || isUnchecked {
				isBullet := strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*")
				isSubHeading := strings.HasPrefix(trimmed, "##")

				if isBullet || isSubHeading {
					results[currentGroup] = append(results[currentGroup], Todo{
						Text:   trimmed,
						IsDone: isChecked,
						Group:  currentGroup,
						Line:   i + 1,
						Weight: DeriveTodoWeight(trimmed),
					})
				}
			}
		}
	}
	return results
}
