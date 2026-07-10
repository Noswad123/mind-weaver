package parser

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveInternalLink takes a raw path like "foo.md"
// and returns a normalized version like "foo/bar"
func ResolveInternalLink(rawPath string) string {
	clean := strings.TrimSpace(rawPath)
	clean = strings.TrimPrefix(clean, "$")
	clean = strings.Split(clean, "#")[0]
	clean = strings.Split(clean, "?")[0]
	clean = strings.TrimSuffix(clean, ".md")
	clean = filepath.Clean(clean)

	return clean
}

func ResolveMarkdownLocalLink(rawPath, sourcePath string) string {
	return ResolveMarkdownLocalLinkWithContext(rawPath, ParseContext{SourceRelPath: sourcePath})
}

func ResolveMarkdownLocalLinkWithContext(rawPath string, ctx ParseContext) string {
	clean := strings.TrimSpace(rawPath)
	clean = strings.Trim(clean, "<>")
	clean = strings.Split(clean, "#")[0]
	clean = strings.Split(clean, "?")[0]
	if clean == "" {
		return ""
	}
	clean = expandHomePath(clean)

	if filepath.IsAbs(clean) {
		if ctx.NotesRootAbs != "" {
			if rel, ok := relIfUnderRoot(clean, ctx.NotesRootAbs); ok {
				return filepath.ToSlash(filepath.Clean(rel))
			}
			return ""
		}
		return filepath.ToSlash(filepath.Clean(clean))
	}

	sourceDir := filepath.Dir(ctx.SourceRelPath)
	if sourceDir == "." {
		sourceDir = ""
	}
	return filepath.ToSlash(filepath.Clean(filepath.Join(sourceDir, clean)))
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func relIfUnderRoot(absTarget, notesRoot string) (string, bool) {
	root := filepath.Clean(expandHomePath(notesRoot))
	target := filepath.Clean(expandHomePath(absTarget))

	rel, err := filepath.Rel(root, target)
	if err != nil {
		return "", false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func NormalizeWikiLinkTarget(raw string) string {
	target := strings.TrimSpace(raw)
	if idx := strings.Index(target, "|"); idx >= 0 {
		target = target[:idx]
	}
	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	return strings.TrimSpace(target)
}
