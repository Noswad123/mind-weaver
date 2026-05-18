package formatter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
	"github.com/Noswad123/mind-weaver/internal/markdown"
)

const hubFileName = notefiles.HubNoteFilename

func buildSnapshot(dirPath string, createMissingChildHubs bool) (DirSnapshot, []string, error) {
	dir := filepath.Clean(dirPath)
	hubPath := filepath.Join(dir, hubFileName)

	hubID := ""
	if b, err := os.ReadFile(hubPath); err == nil {
		id, _ := markdown.ReadMetaIDFromContent(string(b))
		hubID = strings.TrimSpace(id)
	}

	childHubIDs := map[string]bool{}
	relNoteIDs := map[string]bool{}
	issues := []string{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return DirSnapshot{}, nil, fmt.Errorf("readdir %s: %w", dir, err)
	}

	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(dir, name)

		if e.IsDir() {
			childHubPath := filepath.Join(full, hubFileName)

			if createMissingChildHubs {
				if err := ensureHubFile(childHubPath); err != nil {
					issues = append(issues, fmt.Sprintf("failed to ensure child hub %s: %v", childHubPath, err))
				}
			}

			if _, err := os.Stat(childHubPath); err == nil {
				id, ok := markdown.ReadMetaID(childHubPath)
				id = strings.TrimSpace(id)
				if !ok || id == "" {
					id = name
				}
				childHubIDs[id] = true
			}
			continue
		}

		// sibling markdown notes (exclude folder hub note)
		if strings.EqualFold(filepath.Ext(name), ".md") && !strings.EqualFold(name, hubFileName) {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			relNoteIDs[base] = true
		}
	}

	return DirSnapshot{
		DirPath:     dir,
		HubPath:     hubPath,
		HubID:       hubID,
		ChildHubIDs: childHubIDs,
		RelNoteIDs:  relNoteIDs,
	}, issues, nil
}
