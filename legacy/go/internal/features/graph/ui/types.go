package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type tab int
type col int

const (
	tabFocus tab = iota
	tabIndex
)

type NoteLite struct {
	ID    int
	Title string
	Path  string
	Deg   string
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

type GraphService interface {
	SearchIndex(query string, limit, offset int) ([]ConnectednessRow, error)
	LoadFocus(noteID int, neighborsLimit int) (FocusView, error)
	ResolveStartNote(query string) (*NoteLite, error)
	ListIndex(limit, offset int) ([]ConnectednessRow, error) 
}

type loadIndexMsg struct {
	rows []ConnectednessRow
	off  int
}

type loadFocusMsg struct {
	view FocusView
}

type errMsg struct{ err error }
type statusMsg struct{ text string }

type model struct {
	svc GraphService
	initialCmd tea.Cmd

	// tabs
	activeTab tab

	// Focus view state
	focusID     int
	focusTitle  string
	focusPath   string
	inLinks     []NoteLite
	outLinks    []NoteLite
	backCur     int
	outCur      int
	activeCol   col
	showPreview bool
	preview     string
	unresolved  int

	// Index view state
	indexRows []ConnectednessRow
	indexCur  int
	indexOff  int
	pageSize  int

	// Search/filter state (Index tab)
	filtering  bool
	searchText string

	// Status / errors
	status string
	err    error
}
