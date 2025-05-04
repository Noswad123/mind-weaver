package gitutil

import (
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// ValidateGitStatus checks if the file at filePath has no uncommitted changes in the Git repo rooted at repoRoot.
func ValidateGitStatus(filePath, repoRoot string) bool {
	relPath, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		log.Printf("⚠️ Failed to get relative path for git validation: %v\n", err)
		return false
	}

	cmd := exec.Command("git", "status", "--porcelain", relPath)
	cmd.Dir = repoRoot

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ Git status failed for %s: %v\n%s", relPath, err, out.String())
		return false
	}

	if strings.TrimSpace(out.String()) != "" {
		log.Printf("⛔ Skipping %s due to uncommitted Git changes.\n", relPath)
		return false
	}

	return true
}
