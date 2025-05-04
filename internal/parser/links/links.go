package links

import (
	"path/filepath"
	"strings"
)

// ResolveInternalLink takes a raw path like "$foo/bar.norg" or "foo.md"
// and returns a normalized version like "foo/bar"
func ResolveInternalLink(rawPath string) *string {
	clean := strings.TrimSpace(rawPath)
	clean = strings.TrimPrefix(clean, "$")                       // remove "$" prefix if present
	clean = strings.TrimSuffix(clean, ".norg")
	clean = strings.TrimSuffix(clean, ".md")
	clean = filepath.Clean(clean)

	if clean == "" {
		return nil
	}
	return &clean
}
