// shouldn't I pass structs specific to the db and rename db to reflect this like noteDb and cheatdb.
// Create a new sqlite schema for grimmoire (cheatsheets table, etc.)
// Add a second helper.OpenGrimmoireDB() function
// Switch logic in RunTUI based on mode
// In TUI, load cheatsheets as list.Items and wire them to the viewport + textarea for viewing/editing.
// A note metadata query tool (mode == "void")
// spirits summoned from the void
// A cheatsheet manager (mode == "grimmoire")
// incantations/spells summoned from the grimmoire

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

type Cheatsheet struct {
	TitleText   string
	ContentText string
}

func (c Cheatsheet) Title() string       { return c.TitleText }
func (c Cheatsheet) Description() string { return "" }
func (c Cheatsheet) FilterValue() string { return c.TitleText }

type model struct {
	noteDb      *db.NoteDb
	cheatDb     *db.CheatDb
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
	mode        string
}

func initialModel(noteDb *db.NoteDb, cheatDb *db.CheatDb, queries []Query, cheats []Cheatsheet,  mode string) model {
	ta := textarea.New()
	ta.Placeholder = "Enter SQL here..."
	ta.Focus()
	ta.SetWidth(70)
	ta.SetHeight(6)

	var listItems []list.Item
		if mode == "spirits" {
			for _, q := range queries {
				listItems = append(listItems, q)
			}
	} else {
		for _, c:= range cheats {
				listItems = append(listItems, c)
		}
	}

	l := list.New(listItems, list.NewDefaultDelegate(), 30, 10)
	if mode == "spirits" {
		l.Title = "Saved Queries"
	} else {
		l.Title = "Cheatsheets"
	}

	vp := viewport.New(70, 10)
	vp.SetContent("Results will appear here...")

	return model{
		noteDb:     noteDb,
		cheatDb: cheatDb,
		mode:       mode,
		textarea:   ta,
		savedList:  l,
		viewport:   vp,
		queries:    queries,
		cursorMode: "textarea",
	}
}

func (m model) Init() tea.Cmd {
	return textarea.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.mode == "spirits" {
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
			} else if m.mode == "incantations" {
				if m.cursorMode == "textarea" {
					m.viewport.SetContent("Incantation editiing not implemented yet.")
				} else if m.cursorMode == "list" {
					if selected, ok := m.savedList.SelectedItem().(Cheatsheet); ok {
						m.viewport.SetContent(selected.ContentText)
					}
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

func (m model) View() string {
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

func RunTUI(noteDb *db.NoteDb, cheatDb *db.CheatDb, mode string) error {
	defer noteDb.Close()
	defer cheatDb.Close()

	var queries []Query
	var cheats []Cheatsheet

	if mode == "spirits" {
		dbQueries, err := noteDb.LoadSavedQueries()
		if err != nil {
			return err
		}
		for _, q := range dbQueries {
			queries = append(queries, Query{q})
		}
	} else if mode == "incantations" {
		dbCheats, err := cheatDb.LoadCheatsheets()
		if err != nil {
			return err
		}
		for _, ch := range dbCheats {
			cheats = append(cheats, Cheatsheet{TitleText: ch.Name, ContentText: ch.SQL})
		}
	}

	p := tea.NewProgram(initialModel(noteDb, cheatDb, queries, cheats, mode))
	_, err := p.Run()
	return err
}

func errToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
