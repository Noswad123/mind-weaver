package parser

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/parser/meta"
	"github.com/Noswad123/mind-weaver/internal/parser/todo"
	"github.com/Noswad123/mind-weaver/internal/parser/links"
)

type Link struct {
	Type         string  // "internal" or "external"
	Target       string
	Label        string
	ResolvedPath *string
}

type ParsedNote struct {
	Title   string
	Tags    []string
	Todos   []todo.Todo
	Links   []Link
	Content string
}

func ParseNorg(content string, filePath string) ParsedNote {
	title := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	metadata := meta.ExtractMetadata(content)
	metadataTags := metadata.Tags

	tags := make([]string, 0)
	if len(metadataTags) > 0 {
		tags = append(tags, metadataTags...)
	}

	tagPattern := regexp.MustCompile(`:([a-zA-Z0-9_-]+):`)
	matches := tagPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		tags = append(tags, m[1])
	}

	todos := todo.ExtractTodos(content)
	parsedLinks := make([]Link, 0)

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
		resolved := links.ResolveInternalLink(rawPath)
		parsedLinks = append(parsedLinks, Link{
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
		parsedLinks = append(parsedLinks, Link{
			Type:         "external",
			Target:       url,
			Label:        label,
			ResolvedPath: nil,
		})
	}

	return ParsedNote{
		Title:   title,
		Tags:    tags,
		Todos:   todos,
		Links:   parsedLinks,
		Content: content,
	}
}
