package writer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/db"
)
func WriteWorkspaces(db *db.DB, configFilePath, notesRoot string) error {
	paths, err := db.GetWorkspaceNotePaths()
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	entries := GenerateWorkspaceEntries(paths, notesRoot)

	if err := ReplaceWorkspacesBlock(configFilePath, entries); err != nil {
		return err
	}

	log.Println("🔄 Synced Neorg workspaces.")
	return nil
}

func toCamelCase(input string) string {
	input = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(input, " ")
	words := strings.Fields(input)
	for i, word := range words {
		words[i] = strings.Title(word)
	}
	return strings.Join(words, "")
}

func GenerateWorkspaceEntries(paths []string, notesRoot string) []string {
	used := map[string]bool{}
	var entries []string

	for _, path := range paths {
		relDir := strings.TrimPrefix(strings.TrimSuffix(path, "/index.norg"), notesRoot)
		relDir = strings.TrimPrefix(relDir, "/")
		segments := strings.Split(relDir, "/")
		rawName := toCamelCase(segments[len(segments)-1])

		name := rawName
		for count := 1; used[name]; count++ {
			name = fmt.Sprintf("%s%d", rawName, count)
		}
		used[name] = true
		fullPath := filepath.Join(notesRoot, relDir)
		entries = append(entries, fmt.Sprintf("            %s = \"%s\",", name, fullPath))
	}

	sort.Strings(entries)
	return entries
}

func ReplaceWorkspacesBlock(configFilePath string, entries []string) error {
	luaBlock := fmt.Sprintf("workspaces = {\n%s\n          },", strings.Join(entries, "\n"))

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read Lua config: %w", err)
	}

	updated := regexp.MustCompile(`workspaces = \{[\s\S]*?\},`).ReplaceAllString(string(data), luaBlock)
	return os.WriteFile(configFilePath, []byte(updated), 0644)
}
