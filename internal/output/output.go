package output

import (
	"fmt"
	"os"
	"encoding/json"

	"github.com/Noswad123/mind-weaver/internal/parser"
)

type Format string

const (
	FormatPretty Format = "pretty"
	FormatJSON   Format = "json"
	FormatMarkdown Format = "md"
)

// PrintNotes prints notes to the screen in a specified format.
func PrintNotes(notes []parser.ParsedNote, format Format) {
	switch format {
	case FormatJSON:
		printJSON(notes)
	case FormatMarkdown:
		printMarkdown(notes)
	default:
		printPretty(notes)
	}
}

func printPretty(notes []parser.ParsedNote) {
	for _, note := range notes {
		fmt.Printf("📄 %s\n", note.Title)
		fmt.Println("Tags:", note.Tags)
		fmt.Println("Links:", note.Links)
		for _, todo := range note.Todos {
			fmt.Printf("  • [%s] %s\n", todo.RawStatus, *todo.Task)
		}
		fmt.Println("---")
	}
}

func printJSON(notes []parser.ParsedNote) {
	data, err := json.MarshalIndent(notes, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to encode JSON: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

func printMarkdown(notes []parser.ParsedNote) {
	for _, note := range notes {
		fmt.Printf("# %s\n\n", note.Title)
		fmt.Printf("**Tags:** %v\n", note.Tags)
		fmt.Printf("**Links:** %v\n\n", note.Links)
		if len(note.Todos) > 0 {
			fmt.Println("## TODOs")
			for _, todo := range note.Todos {
				fmt.Printf("- [%s] %s\n", todo.RawStatus, *todo.Task)
			}
		}
		fmt.Println("\n---\n")
	}
}
