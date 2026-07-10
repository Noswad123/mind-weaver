package parser

type NoteParser interface {
	Parse(content string, filePath string) (ParsedNote, error)
}

type ParseContext struct {
	SourceRelPath string
	NotesRootAbs  string
}

func ParseNote(content, filePath string) ParsedNote {
	md_parser := MarkdownParser{}

	note, _ := md_parser.Parse(content, filePath)

	return note
}

func ParseNoteWithContext(content string, ctx ParseContext) ParsedNote {
	md_parser := MarkdownParser{}

	note, _ := md_parser.ParseWithContext(content, ctx)

	return note
}
