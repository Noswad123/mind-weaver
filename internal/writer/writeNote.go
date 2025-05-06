package writer

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/formatter"
	"github.com/Noswad123/mind-weaver/internal/parser"
	"github.com/Noswad123/mind-weaver/internal/gitutil"
)

type WriteOptions struct {
	ID               *int
	NotePath         *string
	All              bool
	GenerateIndices  bool
	ConfigFilePath   string
	NotesRoot        string
	DB               *sql.DB
}

func WriteNoteFromDb(noteID int, notesRoot string, db *sql.DB) error {
	var path, title, content string
	err := db.QueryRow(`SELECT path, title, content FROM notes WHERE id = ?`, noteID).Scan(&path, &title, &content)
	if err != nil {
		return fmt.Errorf("note ID %d not found: %w", noteID, err)
	}

	tags := []string{}
	rows, err := db.Query(`SELECT tag FROM tags WHERE note_id = ?`, noteID)
	if err == nil {
		for rows.Next() {
			var tag string
			if err := rows.Scan(&tag); err == nil {
				tags = append(tags, tag)
			}
		}
		rows.Close()
	}

	metaRegex := regexp.MustCompile(`(?m)^@meta\s+[\s\S]*?^@end`)
	if !metaRegex.MatchString(content) && len(tags) > 0 {
		tagLine := fmt.Sprintf("@meta\n  tags = %q\n@end\n\n", strings.Join(tags, ", "))
		content = tagLine + content
	}

	targetPath := filepath.Join(notesRoot, path)
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if !gitutil.ValidateGitStatus(targetPath, notesRoot) {
		return nil // Skip writing if git status is dirty
	}

	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write note %s: %w", targetPath, err)
	}

  note := parser.ParseNote(content, path)
  formatted := formatter.FormatNote(note, targetPath, notesRoot)
  os.WriteFile(targetPath, []byte(formatted), 0644)

	log.Printf("📝 Updated note from DB: %s\n", path)
	return nil
}

func WriteNotesFromDb(opts WriteOptions) {
	if opts.All {
		rows, err := opts.DB.Query(`SELECT id FROM notes`)
		if err != nil {
			log.Fatal("❌ Failed to list notes:", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err == nil {
				WriteNoteFromDb(id, opts.NotesRoot, opts.DB)
			}
		}

		log.Println("✅ All notes updated from DB")
		return
	}

	if opts.ID != nil {
		WriteNoteFromDb(*opts.ID, opts.NotesRoot, opts.DB)
		return
	}

	if opts.NotePath != nil {
		var id int
		err := opts.DB.QueryRow(`SELECT id FROM notes WHERE path = ?`, *opts.NotePath).Scan(&id)
		if err != nil {
			log.Fatalf("❌ No note found with path: %s", *opts.NotePath)
		}
		WriteNoteFromDb(id, opts.NotesRoot, opts.DB)
		return
	}

	log.Fatal("❌ Missing --id, --path, or --all option")
}
