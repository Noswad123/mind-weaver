package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Noswad123/mind-weaver/internal/core/note"
)

type Format string

const (
	FormatPretty   Format = "pretty"
	FormatJSON     Format = "json"
	FormatMarkdown Format = "md"
)

func PrintNotes(notes []note.Note, format Format) {
	switch format {
	case FormatJSON:
		printJSON(notes)
	case FormatMarkdown:
		printMarkdown(notes)
	default:
		printPretty(notes)
	}
}

func printPretty(notes []note.Note) {
	for _, note := range notes {
		fmt.Printf("📄 %s\n", note.Title)
		fmt.Println("Tags:", note.Tags)
		fmt.Println("Links:", note.Links)
		fmt.Println("---")
	}
}

func printJSON(notes []note.Note) {
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to encode JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printMarkdown(notes []note.Note) {
	for _, note := range notes {
		fmt.Printf("# %s\n\n", note.Title)
		fmt.Printf("**Tags:** %v\n", note.Tags)
		fmt.Printf("**Links:** %v\n\n", note.Links)
		fmt.Println("\n---")
	}
}
