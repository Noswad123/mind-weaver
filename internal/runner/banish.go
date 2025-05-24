package runner


import (
	"log"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/writer"
	"github.com/Noswad123/mind-weaver/internal/helper"
)

func RunBanishCommand(c *cli.Context, env helper.Config, db *db.DB) error {
		log.Println("🔁 Sync all notes...")
		files := []string{}
		err := filepath.Walk(env.NotesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && (filepath.Ext(path) == ".norg" || filepath.Ext(path) == ".md") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error walking notes directory: %v", err)
		}

		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				log.Printf("❌ Could not read %s: %v", f, err)
				continue
			}
			note := parser.ParseNote(string(content), f)
			db.UpsertNote(note, f)
		}
		log.Printf("✅ synced %d notes\n", len(files))

		if err := writer.WriteWorkspaces(db, env.ConfigPath, env.NotesDir); err != nil {
			log.Printf("⚠️ Failed to sync Neorg workspaces: %v\n", err)
		}
	return nil
}
