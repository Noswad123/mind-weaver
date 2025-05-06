package formatter

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

func FormatIndexNote(dirPath, notesRoot string) {
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

func insertTopics(content string, dirPath string) string {
	if !strings.Contains(content, "* Topics") {
    log.Printf("⚠️ No * Topics header found in %s", dirPath)
		return content
	}

	existingLinks := extractLinks(content)
	dirLinks := buildLinksFromDir(dirPath, existingLinks)

	if len(dirLinks) > 0 {
		content = insertLinks(content, dirLinks)
	}

	content = cleanupStaleLinks(content, dirPath)

	return content
}

func extractLinks(content string) map[string]bool {
	linkPattern := regexp.MustCompile(`\*\*\s+\{(:|\$:)([^}/]+)(/index)?:\}`)
	links := make(map[string]bool)
	for _, match := range linkPattern.FindAllStringSubmatch(content, -1) {
		if match[1] == "$" && match[3] == "/index" {
			links[match[2]+"/index"] = true
		} else {
			links[match[2]] = true
		}
	}
	return links
}

func buildLinksFromDir(dirPath string, existing map[string]bool) []string {
	entries := []string{}

	filesAndFolders, err := os.ReadDir(dirPath)
	if err != nil {
		return entries
	}

	for _, fileOrFolder := range filesAndFolders {
		name := fileOrFolder.Name()
		fullPath := filepath.Join(dirPath, name)

		if fileOrFolder.IsDir() {
			indexPath := filepath.Join(fullPath, "index.norg")
			if _, err := os.Stat(indexPath); err == nil {
				key := name + "/index"
				if !existing[key] {
					entries = append(entries, fmt.Sprintf("** {:$%s/index:}", name))
				}
			}
		} else if filepath.Ext(name) == ".norg" && name != "index.norg" {
			base := strings.TrimSuffix(name, ".norg")
			if !existing[base] {
				entries = append(entries, fmt.Sprintf("** {:%s:}", base))
			}
		}
	}

	sort.Strings(entries)
	return entries
}

func insertLinks(content string, entries []string) string {
	lines := strings.Split(content, "\n")
	out := []string{}
	inserted := false

	for _, line := range lines {
		out = append(out, line)
		if !inserted && strings.TrimSpace(line) == "* Topics" {
			out = append(out, entries...)
			inserted = true
		}
	}
	return strings.Join(out, "\n")
}

func pathMissing(paths ...string) bool {
	_, err := os.Stat(filepath.Join(paths...))
	return err != nil
}

func cleanupStaleLinks(content, dirPath string) string {
	linkPattern := regexp.MustCompile(`\*\*\s+\{(:|\$:)([^}/]+)(/index)?:\}`)
	lines := strings.Split(content, "\n")
	cleaned := []string{}

	for _, line := range lines {
		if match := linkPattern.FindStringSubmatch(line); match != nil {
			name := match[2]
      indexPatternMatched := match[1] == "$" && match[3] == "/index"
      filePathMissing := indexPatternMatched &&  pathMissing(dirPath, name, "index.norg") 
      folderPathMissing := !indexPatternMatched && pathMissing(dirPath, name+".norg")
			missing := filePathMissing || folderPathMissing

			if missing {
        log.Printf("Cleaning stale link: %s\n", name)
				line = "** " + name
			}
		}
		cleaned = append(cleaned, line)
	}
  return strings.Join(cleaned, "\n")
}
