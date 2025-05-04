package indexer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

  "github.com/Noswad123/mind-weaver/internal/gitutil"
)

var requiredHeaders = []string{
	"* Todo",
	"* Topics",
	"* Research",
	"* Resources",
}

func EnsureIndex(dirPath, notesRoot string) {
	indexPath := filepath.Join(dirPath, "index.norg")
	relative, _ := filepath.Rel(notesRoot, indexPath)
	if relative == "index.norg" {
		return // skip root index.norg
	}

	existing := ""
	if _, err := os.Stat(indexPath); err == nil {
		content, err := os.ReadFile(indexPath)
		if err == nil {
			existing = string(content)
		}
	}

	updated := existing
	if !regexp.MustCompile(`(?m)^@meta[\s\S]*?^@end`).MatchString(existing) {
		updated = "@meta\ntags: []\n@end\n\n" + updated
	}

	for _, header := range requiredHeaders {
		if !strings.Contains(updated, header) {
			updated += "\n" + header
		}
	}

	updated = insertTopics(updated, dirPath)

	if !gitutil.ValidateGitStatus(indexPath, notesRoot) {
		return
	}

	if err := os.WriteFile(indexPath, []byte(strings.TrimSpace(updated)+"\n"), 0644); err != nil {
		log.Printf("❌ Failed to write index.norg at %s: %v", relative, err)
	} else {
		log.Printf("📁 Ensured index.norg: %s", relative)
	}
}

func insertTopics(content, dirPath string) string {
	topicsHeader := "* Topics"
	if !strings.Contains(content, topicsHeader) {
		return content
	}

	existingLinks := map[string]bool{}
	linkPattern := regexp.MustCompile(`\*\* \{(:|\$:)([^}]+):\}`)
	for _, match := range linkPattern.FindAllStringSubmatch(content, -1) {
		existingLinks[match[2]] = true
	}

	entries := []string{}
	items, _ := os.ReadDir(dirPath)
	for _, item := range items {
		name := item.Name()
		if item.IsDir() && !existingLinks[name+"/index"] {
			entries = append(entries, fmt.Sprintf("** {:$%s/index:}", name))
		} else if filepath.Ext(name) == ".norg" && name != "index.norg" {
			base := strings.TrimSuffix(name, ".norg")
			if !existingLinks[base] {
				entries = append(entries, fmt.Sprintf("** {:%s:}", base))
			}
		}
	}
	sort.Strings(entries)

	// Inject topic links after * Topics
	lines := strings.Split(content, "\n")
	out := []string{}
	inserted := false
	for _, line := range lines {
		out = append(out, line)
		if !inserted && strings.TrimSpace(line) == topicsHeader {
			out = append(out, entries...)
			inserted = true
		}
	}
	return strings.Join(out, "\n")
}
