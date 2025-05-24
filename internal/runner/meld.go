package runner

import (
	"github.com/Noswad123/mind-weaver/internal/formatter"
	"github.com/Noswad123/mind-weaver/internal/helper"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"path/filepath"
)

func RunMeldCommand(c *cli.Context, env helper.Config)error {
	log.Println("🧩 Ensuring index.norg files exist and are structured...")
	entries, err := os.ReadDir(env.NotesDir)
	if err != nil {
		return cli.Exit("❌ Failed to list notes directory: " + err.Error(), 1)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(env.NotesDir, entry.Name())
			indexPath := filepath.Join(dir, "index.norg")
			if _, err := os.Stat(indexPath); os.IsNotExist(err) {
				log.Printf("➕ Creating missing index.norg in %s", dir)
				os.WriteFile(indexPath, []byte(""), 0644)
			}
			formatter.FormatIndexNote(dir, env.NotesDir)
		}
	}
	return nil
}
