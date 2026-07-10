package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	theme "github.com/Noswad123/mind-weaver/internal/shared/ui"
)


const (
	colBacklinks col = iota
	colOutlinks
)


func Run(svc GraphService, initialQuery string) error {
	m := model{
		svc:         svc,
		activeTab:   tabIndex,
		pageSize:    50,
		showPreview: true,
		status:      "Loading…",
	}

	p := tea.NewProgram(m)

	m.initialCmd = tea.Batch(
		m.loadIndexCmd(0),
		m.resolveStartCmd(initialQuery),
	)

	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	_ = finalModel
	return nil
}

func (m model) Init() tea.Cmd { return nil }


func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case loadIndexMsg:
		m.indexRows = msg.rows
		m.indexOff = msg.off
		m.indexCur = 0
		if len(m.indexRows) == 0 {
			m.status = "No results."
		} else {
			m.status = ""
		}
		return m, nil

	case loadFocusMsg:
		v := msg.view
		m.focusID = v.Focus.ID
		m.focusTitle = v.Focus.Title
		m.focusPath = v.Focus.Path
		m.preview = v.Preview
		m.unresolved = v.Unresolved
		m.inLinks = v.Backlinks
		m.outLinks = v.Outlinks
		m.backCur, m.outCur = 0, 0
		m.activeCol = colBacklinks
		m.activeTab = tabFocus
		m.status = ""
		return m, nil

	case errMsg:
		m.err = msg.err
		m.status = "Error: " + msg.err.Error()
		return m, nil

	case statusMsg:
		m.status = msg.text
		return m, nil

	case tea.KeyMsg:
		k := msg.String()

		if m.filtering {
			switch k {
			case "esc":
				m.filtering = false
				m.searchText = ""
				m.indexOff = 0
				return m, m.loadIndexCmd(0)

			case "enter":
				m.filtering = false
				m.indexOff = 0
				if strings.TrimSpace(m.searchText) == "" {
					return m, m.loadIndexCmd(0)
				}
				return m, m.searchIndexCmd(m.searchText, 0)

			case "backspace":
				if len(m.searchText) > 0 {
					m.searchText = m.searchText[:len(m.searchText)-1]
				}
				return m, nil

			default:
				if len(k) == 1 {
					m.searchText += k
				}
				return m, nil
			}
		}

		switch k {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			if m.activeTab == tabFocus {
				m.activeTab = tabIndex
			} else {
				m.activeTab = tabFocus
			}
			return m, nil

		case "/":
			if m.activeTab == tabIndex {
				m.filtering = true
				m.searchText = ""
				return m, nil
			}
			return m, nil

		case "p":
			m.showPreview = !m.showPreview
			return m, nil

		case "up", "k":
			return m.moveUp(), nil
		case "down", "j":
			return m.moveDown(), nil

		case "h":
			if m.activeTab == tabFocus {
				m.activeCol = colBacklinks
			}
			return m, nil
		case "l":
			if m.activeTab == tabFocus {
				m.activeCol = colOutlinks
			}
			return m, nil

		case "enter":
			return m.activate()

		case "]":
			// next page index
			if m.activeTab == tabIndex {
				next := m.indexOff + m.pageSize
				if strings.TrimSpace(m.searchText) != "" {
					return m, m.searchIndexCmd(m.searchText, next)
				}
				return m, m.loadIndexCmd(next)
			}
			return m, nil
		case "[":
			if m.activeTab == tabIndex {
				prev := m.indexOff - m.pageSize
				if prev < 0 {
					prev = 0
				}
				if strings.TrimSpace(m.searchText) != "" {
					return m, m.searchIndexCmd(m.searchText, prev)
				}
				return m, m.loadIndexCmd(prev)
			}
			return m, nil
		}
	}

	return m, nil
}

func (m model) moveUp() model {
	switch m.activeTab {
	case tabIndex:
		if m.indexCur > 0 {
			m.indexCur--
		}
	case tabFocus:
		if m.activeCol == colBacklinks && m.backCur > 0 {
			m.backCur--
		}
		if m.activeCol == colOutlinks && m.outCur > 0 {
			m.outCur--
		}
	}
	return m
}

func (m model) moveDown() model {
	switch m.activeTab {
	case tabIndex:
		if m.indexCur < len(m.indexRows)-1 {
			m.indexCur++
		}
	case tabFocus:
		if m.activeCol == colBacklinks && m.backCur < len(m.inLinks)-1 {
			m.backCur++
		}
		if m.activeCol == colOutlinks && m.outCur < len(m.outLinks)-1 {
			m.outCur++
		}
	}
	return m
}

func (m model) activate() (tea.Model, tea.Cmd) {
	switch m.activeTab {
	case tabIndex:
		if len(m.indexRows) == 0 {
			return m, nil
		}
		id := m.indexRows[m.indexCur].Note.ID
		return m, m.loadFocusCmd(id)

	case tabFocus:
		var id int
		if m.activeCol == colBacklinks {
			if len(m.inLinks) == 0 {
				return m, nil
			}
			id = m.inLinks[m.backCur].ID
		} else {
			if len(m.outLinks) == 0 {
				return m, nil
			}
			id = m.outLinks[m.outCur].ID
		}
		return m, m.loadFocusCmd(id)
	}
	return m, nil
}

// --- view
func (m model) View() string {
	var b strings.Builder
	b.WriteString("\n")

	// Tabs
	tabs := []string{"Focus", "Index"}
	var renderedTabs []string
	for i, t := range tabs {
		style := theme.TabStyle
		if int(m.activeTab) == i {
			style = theme.ActiveTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(t))
	}
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...) + "\n\n")

	// Filter prompt
	if m.filtering {
		b.WriteString(theme.LabelStyle.Render("Filter") + " " + theme.CursorStyle.Render(m.searchText+"█") + "\n\n")
	}

	// Status / error
	if m.status != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.Subtle).Render(m.status) + "\n\n")
	}

	switch m.activeTab {
	case tabIndex:
		b.WriteString(m.renderIndex())
	case tabFocus:
		b.WriteString(m.renderFocus())
	}

	help := "↑/↓: Navigate • Enter: Select • Tab: Switch • /: Filter(Index) • [ ]: Page(Index) • h/l: Column(Focus) • p: Preview • q: Quit"
	b.WriteString("\n\n" + lipgloss.NewStyle().Foreground(theme.Subtle).Render(help) + "\n")
	return b.String()
}

func (m model) renderIndex() string {
	if len(m.indexRows) == 0 {
		return lipgloss.NewStyle().Italic(true).Foreground(theme.Subtle).Render("No notes to display.")
	}

	title := theme.TitleStyle.Render(fmt.Sprintf("Connectedness Index  (offset %d)", m.indexOff))

	var s strings.Builder
	for i, row := range m.indexRows {
		cursor := "  "
		if i == m.indexCur {
			cursor = theme.CursorStyle.Render("> ")
		}
		total := row.In + row.Out
		right := lipgloss.NewStyle().Foreground(theme.Subtle).Render(fmt.Sprintf("in:%d out:%d total:%d", row.In, row.Out, total))
		line := fmt.Sprintf("%s%-44s %s", cursor, row.Note.Title, right)
		s.WriteString(line + "\n")
	}

	return title + "\n" + s.String()
}

func (m model) renderFocus() string {
	if m.focusID == 0 {
		return lipgloss.NewStyle().Italic(true).Foreground(theme.Subtle).Render("No focused note. Go to Index and select one.")
	}

	header := theme.TitleStyle.Render(fmt.Sprintf("%s  %s", m.focusTitle, lipgloss.NewStyle().Foreground(theme.Subtle).Render(m.focusPath)))
	if m.unresolved > 0 {
		header += "\n" + lipgloss.NewStyle().Foreground(theme.Alert).Render(fmt.Sprintf("Unresolved internal links: %d", m.unresolved))
	}

	left := m.renderNoteList("Backlinks", m.inLinks, m.backCur, m.activeCol == colBacklinks)
	right := m.renderNoteList("Outlinks", m.outLinks, m.outCur, m.activeCol == colOutlinks)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top,
		panelStyle().Width(46).Render(left),
		panelStyle().Width(46).Render(right),
	)

	if !m.showPreview {
		return header + "\n" + topRow
	}

	preview := m.renderPreview()
	return header + "\n" + topRow + "\n" + panelStyle().Width(94).Render(preview)
}

func (m model) renderNoteList(label string, notes []NoteLite, cursor int, active bool) string {
	head := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Render(fmt.Sprintf("%s (%d)", label, len(notes)))
	if len(notes) == 0 {
		return head + "\n" + lipgloss.NewStyle().Italic(true).Foreground(theme.Subtle).Render("None")
	}

	var s strings.Builder
	for i, n := range notes {
		prefix := "  "
		if active && i == cursor {
			prefix = theme.CursorStyle.Render("> ")
		}
		line := fmt.Sprintf("%s%s %s", prefix, n.Title, lipgloss.NewStyle().Foreground(theme.Subtle).Render(n.Deg))
		s.WriteString(line + "\n")
	}
	return head + "\n" + s.String()
}

func (m model) renderPreview() string {
	head := lipgloss.NewStyle().Bold(true).Foreground(theme.Accent).Render("Preview")
	if strings.TrimSpace(m.preview) == "" {
		return head + "\n" + lipgloss.NewStyle().Italic(true).Foreground(theme.Subtle).Render("No content.")
	}
	return head + "\n" + m.preview
}

func panelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(theme.Subtle).
		Padding(0, 1)
}
