package updater

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func toCamelCase(input string) string {
	input = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(input, " ")
	words := strings.Fields(input)
	for i, word := range words {
		words[i] = strings.Title(word)
	}
	return strings.Join(words, "")
}

func SyncNeorgWorkspaces(db *sql.DB, configFilePath string, notesRoot string) error {
	rows, err := db.Query(`SELECT path FROM notes WHERE path LIKE '%/index.norg'`)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	usedNames := map[string]bool{}
	entries := []string{}

	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			continue
		}
		if path == "index.norg" {
			continue
		}
		relDir := strings.TrimSuffix(path, "/index.norg")
		segments := strings.Split(relDir, "/")
		rawName := toCamelCase(segments[len(segments)-1])

		name := rawName
		count := 1
		for usedNames[name] {
			name = fmt.Sprintf("%s%d", rawName, count)
			count++
		}
		usedNames[name] = true
		fullPath := filepath.Join(notesRoot, relDir)
		entries = append(entries, fmt.Sprintf("            %s = \"%s\",", name, fullPath))
	}

	sort.Strings(entries)
	luaBlock := fmt.Sprintf("workspaces = {\n%s\n          },", strings.Join(entries, "\n"))

	// Replace workspaces block in Lua config
	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read Lua config: %w", err)
	}

	updated := regexp.MustCompile(`workspaces = \{[\s\S]*?\},`).ReplaceAllString(string(data), luaBlock)
	if err := os.WriteFile(configFilePath, []byte(updated), 0644); err != nil {
		return fmt.Errorf("failed to write updated Lua config: %w", err)
	}

	log.Println("🔄 Synced Neorg workspaces.")
	return nil
}
