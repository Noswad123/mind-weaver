package parser

import (
	"path/filepath"
	"strings"
)

// ResolveInternalLink takes a raw path like "foo.md"
// and returns a normalized version like "foo/bar"
func ResolveInternalLink(rawPath string) string {
	clean := strings.TrimSpace(rawPath)
	clean = strings.TrimPrefix(clean, "$")
	clean = strings.TrimSuffix(clean, ".md")
	clean = filepath.Clean(clean)

	return clean
}
