package watch

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type NotePathLister interface {
	ListWorkspaceNotePaths(ctx context.Context) ([]string, error)
}

type SyncService struct {
	lister NotePathLister
}

func New(lister NotePathLister) *SyncService {
	return &SyncService{lister: lister}
}

func (s *SyncService) Sync(ctx context.Context, configFilePath, notesRoot string) error {
	paths, err := s.lister.ListWorkspaceNotePaths(ctx)
	if err != nil {
		return fmt.Errorf("list workspace note paths: %w", err)
	}

	entries := generateWorkspaceEntries(paths, notesRoot)
	if err := replaceWorkspacesBlock(configFilePath, entries); err != nil {
		return err
	}
	return nil
}

func generateWorkspaceEntries(paths []string, notesRoot string) []string {
	type workspaceCandidate struct {
		relDir string
		isHub  bool
	}

	bestByDir := map[string]workspaceCandidate{}
	for _, path := range paths {
		relDir := notefiles.TrimHubNoteSuffix(path)
		relDir = strings.TrimPrefix(relDir, notesRoot)
		relDir = strings.TrimPrefix(relDir, "/")
		if relDir == "" {
			continue
		}

		candidate := workspaceCandidate{relDir: relDir, isHub: notefiles.IsHubNotePath(path)}
		prev, exists := bestByDir[relDir]
		if !exists || (candidate.isHub && !prev.isHub) {
			bestByDir[relDir] = candidate
		}
	}

	relDirs := make([]string, 0, len(bestByDir))
	for relDir := range bestByDir {
		relDirs = append(relDirs, relDir)
	}
	sort.Strings(relDirs)

	used := map[string]bool{}
	entries := make([]string, 0, len(relDirs))
	for _, relDir := range relDirs {
		segments := strings.Split(relDir, "/")
		rawName := toCamelCase(segments[len(segments)-1])

		name := strings.ReplaceAll(rawName, " ", "")
		for count := 1; used[name]; count++ {
			name = fmt.Sprintf("%s%d", rawName, count)
		}
		used[name] = true
		entries = append(entries, fmt.Sprintf("            %s = notes_dir .. \"/%s\",", name, relDir))
	}

	return entries
}

func replaceWorkspacesBlock(configFilePath string, entries []string) error {
	luaBlock := fmt.Sprintf("workspaces = {\n%s\n          },", strings.Join(entries, "\n"))

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to read Lua config: %w", err)
	}

	updated := regexp.MustCompile(`workspaces = \{[\s\S]*?\},`).ReplaceAllString(string(data), luaBlock)
	return os.WriteFile(configFilePath, []byte(updated), 0644)
}

func toCamelCase(input string) string {
	input = regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(input, " ")
	words := strings.Fields(input)
	c := cases.Title(language.Und)
	for i, word := range words {
		words[i] = c.String(word)
	}
	return strings.Join(words, "")
}
