package graph

import "github.com/Noswad123/mind-weaver/internal/core/shared"

type NoteLite struct {
	ID    shared.ID
	Title string
	Path  string
	Deg   string // UI-friendly; optional
}

type FocusView struct {
	Focus      NoteLite
	Backlinks  []NoteLite
	Outlinks   []NoteLite
	Preview    string
	Unresolved int
}

type ConnectednessRow struct {
	Note NoteLite
	In   int
	Out  int
}
