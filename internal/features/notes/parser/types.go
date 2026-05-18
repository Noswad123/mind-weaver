package parser

import "github.com/Noswad123/mind-weaver/internal/core/note"

type Todo struct {
	Group        string
	DerivedGroup *string
	Task         *string
	Status       string
	RawStatus    string
	Level        int
	Depth        int
	Line         int
	IsGroup      bool
	Text         string
	IsDone       bool
	Weight       float64
	MetaSummary  string
}

type ParsedNote struct {
	Title   string
	Domains []string
	Tags    []string
	Todos   []Todo
	Links   []note.Link
	Content string
}
