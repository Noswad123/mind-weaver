package interactive

import (
	"strings"

	"github.com/Noswad123/mind-weaver/internal/db"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3"
)

type Query struct {
	db.Query // Embed
}

func (q Query) Title() string       { return q.Name }
func (q Query) Description() string { return q.SQL }
func (q Query) FilterValue() string { return q.Name }

type noteModel struct {
	noteDb      *db.NoteDb
	textarea    textarea.Model
	savedList   list.Model
	viewport    viewport.Model
	queries     []Query
	errorMsg    string
	cursorMode  string // textarea | list | viewport
	visualMode  bool
	visualStart int
	visualEnd   int
	yankMessage string
	width       int
	height      int
}

func initialNoteModel(noteDb *db.NoteDb, queries []Query) noteModel {
	ta := textarea.New()
	ta.Placeholder = "Enter SQL here..."
	ta.Focus()
	ta.SetWidth(70)
	ta.SetHeight(6)

	var listItems []list.Item
	for _, q := range queries {
		listItems = append(listItems, q)
	}

	l := list.New(listItems, list.NewDefaultDelegate(), 30, 10)
	l.Title = "Saved Queries"

	vp := viewport.New(70, 10)
	vp.SetContent("Results will appear here...")

	return noteModel{
		noteDb:     noteDb,
		textarea:   ta,
		savedList:  l,
		viewport:   vp,
		queries:    queries,
		cursorMode: "textarea",
	}
}

func (m noteModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m noteModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			switch m.cursorMode {
			case "textarea":
				m.cursorMode = "list"
				m.textarea.Blur()
			case "list":
				m.cursorMode = "viewport"
			case "viewport":
				m.cursorMode = "textarea"
				m.textarea.Focus()
			}
			return m, nil

		case "enter":
			if m.cursorMode == "textarea" {
				query := m.textarea.Value()
				result, err := m.noteDb.ExecuteSQL(query)
				m.viewport.SetContent(result)
				if err != nil {
					m.errorMsg = err.Error()
				} else {
					m.errorMsg = ""
				}
			} else if m.cursorMode == "list" {
				if selected, ok := m.savedList.SelectedItem().(Query); ok {
					m.textarea.SetValue(selected.SQL)
					m.cursorMode = "textarea"
					m.textarea.Focus()
				}
			}
		case "v":
			if m.cursorMode == "viewport" {
				m.visualMode = !m.visualMode
				if m.visualMode {
					m.visualStart = m.viewport.YOffset
					m.visualEnd = m.visualStart
				}
			}

		case "y":
			if m.cursorMode == "viewport" && m.visualMode {
				content := m.viewport.View()
				lines := strings.Split(content, "\n")
				start := m.visualStart
				end := m.visualEnd
				if start > end {
					start, end = end, start
				}
				if start < 0 {
					start = 0
				}
				if end >= len(lines) {
					end = len(lines) - 1
				}
				selection := strings.Join(lines[start:end+1], "\n")
				_ = clipboard.WriteAll(selection)
				m.visualMode = false
				m.yankMessage = "Yanked to clipboard!"
			}

		case "ctrl+u":
			if m.cursorMode == "viewport" {
				m.viewport.ScrollUp(m.viewport.Height / 2)
				if m.visualMode {
					m.visualEnd = m.viewport.YOffset
				}
			}

		case "ctrl+d":
			if m.cursorMode == "viewport" {
				m.viewport.ScrollDown(m.viewport.Height / 2)
				if m.visualMode {
					m.visualEnd = m.viewport.YOffset
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 35)
		m.savedList.SetSize(30, msg.Height-5)
		m.viewport.Width = msg.Width - 35
		m.viewport.Height = msg.Height - 15
	}

	var cmds []tea.Cmd

	switch m.cursorMode {
	case "textarea":
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	case "list":
		var cmd tea.Cmd
		m.savedList, cmd = m.savedList.Update(msg)
		cmds = append(cmds, cmd)
	case "viewport":
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}
	if m.yankMessage != "" {
		m.yankMessage = ""
	}
	return m, tea.Batch(cmds...)
}

func (m noteModel) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().Bold(true).Render("Interactive SQL TUI") + "\n\n"

	left := m.textarea.View() + "\n\n"
	if m.errorMsg != "" {
		left += lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render("Error: " + m.errorMsg)
	}

	lines := strings.Split(m.viewport.View(), "\n")
	if m.visualMode {
		start := m.visualStart
		end := m.visualEnd
		if start > end {
			start, end = end, start
		}
		for i := start; i <= end && i < len(lines); i++ {
			lines[i] = lipgloss.NewStyle().Background(lipgloss.Color("57")).Render(lines[i])
		}
	}
	vp := strings.Join(lines, "\n")
	left += "\n\n" + vp
	layout := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(m.width-32).Render(left),
		lipgloss.NewStyle().Width(30).Border(lipgloss.NormalBorder()).Render(m.savedList.View()),
	)

	footer := "\n[Tab] Switch Focus | [Enter] Run/Select | [q] Quit"
	b.WriteString(header)
	b.WriteString(layout)
	b.WriteString(footer)

	if m.yankMessage != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render(m.yankMessage))
	}

	return b.String()
}

func RunNoteTUI(noteDb *db.NoteDb) error {
	defer noteDb.Close()

	var queries []Query

	dbQueries, err := noteDb.LoadSavedQueries()
	if err != nil {
		return err
	}
	for _, q := range dbQueries {
		queries = append(queries, Query{q})
	}

	p := tea.NewProgram(initialNoteModel(noteDb, queries))
	_, err = p.Run()
	return err
}

func errToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
