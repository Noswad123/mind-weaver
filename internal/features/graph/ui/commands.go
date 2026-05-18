package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)


func (m model) loadIndexCmd(offset int) tea.Cmd {
	limit := m.pageSize
	return func() tea.Msg {
		rows, err := m.svc.ListIndex(limit, offset)
		if err != nil {
			return errMsg{err}
		}
		return loadIndexMsg{rows: rows, off: offset}
	}
}

func (m model) searchIndexCmd(query string, offset int) tea.Cmd {
	limit := m.pageSize
	return func() tea.Msg {
		rows, err := m.svc.SearchIndex(query, limit, offset)
		if err != nil {
			return errMsg{err}
		}
		return loadIndexMsg{rows: rows, off: offset}
	}
}

func (m model) loadFocusCmd(noteID int) tea.Cmd {
	return func() tea.Msg {
		view, err := m.svc.LoadFocus(noteID, 50)
		if err != nil {
			return errMsg{err}
		}
		return loadFocusMsg{view: view}
	}
}

func (m model) resolveStartCmd(query string) tea.Cmd {
	return func() tea.Msg {
		n, err := m.svc.ResolveStartNote(query)
		if err != nil {
			return errMsg{err}
		}
		if n == nil {
			return statusMsg{text: "No notes found."}
		}
		// Load focus async
		return statusMsg{text: fmt.Sprintf("Resolved start: %s", n.Title)}
	}
}
