package notefiles

import (
	"path/filepath"
	"strings"
)

const HubNoteFilename = "hub.md"

func IsHubNotePath(path string) bool {
	return strings.EqualFold(filepath.Base(path), HubNoteFilename)
}

func PreferredHubNotePath(dir string) string {
	return filepath.Join(dir, HubNoteFilename)
}

func TrimHubNoteSuffix(path string) string {
	trimmed := strings.TrimSpace(path)
	lower := strings.ToLower(trimmed)

	if strings.HasSuffix(lower, "/"+HubNoteFilename) {
		return trimmed[:len(trimmed)-len("/"+HubNoteFilename)]
	}
	if strings.EqualFold(trimmed, HubNoteFilename) {
		return ""
	}

	return trimmed
}
