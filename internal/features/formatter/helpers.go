package formatter

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
)

func DedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, item := range items {
		if _, ok := seen[item]; !ok {
			seen[item] = struct{}{}
			out = append(out, item)
		}
	}
	return out
}

func PathMissing(paths ...string) bool {
	full := filepath.Join(paths...)
	_, err := os.Stat(full)
	if os.IsNotExist(err) {
		log.Printf("❌ Path missing: %s", full)
		return true
	}
	return false
}

func EnsureHeaders(lines []string, headers []string) []string {
	seen := make(map[string]bool)
	for _, line := range lines {
		seen[strings.TrimSpace(line)] = true
	}

	for _, header := range headers {
		if !seen[header] {
			// ensure blank line before header
			if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
				lines = append(lines, "")
			}
			lines = append(lines, header)
			lines = append(lines, "") // blank line after header
		}
	}

	return lines
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func IsHubFile(path string) bool {
	return notefiles.IsHubNotePath(path)
}
