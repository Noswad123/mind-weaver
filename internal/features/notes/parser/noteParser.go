package parser

type NoteParser interface {
	Parse(content string, filePath string) (ParsedNote, error)
}

func ParseNote(content, filePath string) (ParsedNote){
	md_parser := MarkdownParser{}

	note, _ := md_parser.Parse(content, filePath)

	return note
}

