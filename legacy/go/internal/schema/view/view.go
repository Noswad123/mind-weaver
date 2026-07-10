package view

import (
	"regexp"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
)

type CodeBlock struct {
	Lang string `json:"lang,omitempty"`
	Code string `json:"code"`
}

type SectionView struct {
	Title      string     `json:"title,omitempty"`
	Bullets    []string   `json:"bullets,omitempty"`
	Paragraphs []string   `json:"paragraphs,omitempty"`
	CodeBlocks []CodeBlock `json:"code_blocks,omitempty"`
	Children   []SectionView `json:"children,omitempty"`
}

type DomainView struct {
	UID     string             `json:"uid"`
	Path    string             `json:"path"`
	Title   string             `json:"title"`
	Domain  string             `json:"domain"`
	Meta    map[string]any     `json:"meta,omitempty"`
	Sections map[string]SectionView `json:"sections"`
}

var metaBlockRe = regexp.MustCompile(`(?ms)^\s*@meta\b.*?^\s*@end\b\s*\n?`)

// StripMeta removes the @meta ... @end block so ParseAST won't accidentally treat it as content.
func StripMeta(content string) string {
	return metaBlockRe.ReplaceAllString(content, "")
}

func BuildSections(ast parser.ASTNode) map[string]SectionView {
	out := map[string]SectionView{}

	for _, child := range ast.Children {
		if child.Type != "heading" || child.Level != 1 {
			continue
		}

		secName := strings.TrimSpace(child.Text)
		if secName == "" {
			continue
		}

		out[secName] = buildHeadingView(child)
	}

	return out
}

func buildHeadingView(h parser.ASTNode) SectionView {
	v := SectionView{Title: strings.TrimSpace(h.Text)}
	// Everything directly under this heading:
	// - bullets/paragraph/code go into this node
	// - subheadings become Children (recursively)
	for _, ch := range h.Children {
		switch ch.Type {
		case "bullet":
			txt := strings.TrimSpace(ch.Text)
			if txt != "" {
				v.Bullets = append(v.Bullets, txt)
			}
		case "paragraph":
			txt := strings.TrimSpace(ch.Text)
			if txt != "" {
				v.Paragraphs = append(v.Paragraphs, txt)
			}
		case "code":
			// Preserve code exactly
			v.CodeBlocks = append(v.CodeBlocks, CodeBlock{
				Lang: strings.TrimSpace(ch.Lang),
				Code: ch.Code,
			})
		case "heading":
			v.Children = append(v.Children, buildHeadingView(ch))
		default:
			// ignore unknown node types
		}
	}
	return v
}
