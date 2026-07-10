package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	_ "github.com/mattn/go-sqlite3"
)

type model struct {
	db          *db.CommandDb
	textarea    textarea.Model
	viewport    viewport.Model
	errorMsg    string
	cursorMode  string // textarea | tools | commands | viewport
	visualMode  bool
	visualStart int
	visualEnd   int
	yankMessage string
	width       int
	height      int

	toolsList    list.Model
	commandsList list.Model
	selectedTool string
	allCommands  []CommandItem
}

func RunTUI(db *db.CommandDb) error {
	defer db.Close()

	rawCommands, err := db.GetAllCommandsWithExamples()
	if err != nil {
		return err
	}

	var commandItems []CommandItem
	for _, c := range rawCommands {
		commandItems = append(commandItems, CommandItem{
			Command:  c.Command,
			Examples: c.Examples,
		})
	}

	ta := textarea.New()
	ta.Placeholder = "Enter command or search..."
	ta.Focus()
	ta.SetHeight(3)
	ta.SetWidth(70)

	toolMap := make(map[string]bool)
	var toolItems []list.Item
	for _, c := range commandItems {
		if !toolMap[c.Command.ToolName] {
			toolMap[c.Command.ToolName] = true
			toolItems = append(toolItems, ToolItem{Name: c.Command.ToolName})
		}
	}

	toolsList := list.New(toolItems, list.NewDefaultDelegate(), 30, 10)
	toolsList.Title = "Tools"

	commandsList := list.New([]list.Item{}, list.NewDefaultDelegate(), 30, 10)
	commandsList.Title = "Commands"

	vp := viewport.New(70, 10)
	vp.SetContent("Select a tool to view commands...")

	mdl := model{
		db:           db,
		textarea:     ta,
		viewport:     vp,
		cursorMode:   "tools",
		toolsList:    toolsList,
		commandsList: commandsList,
		allCommands:  commandItems,
	}

	// Optional: preload commands for the first tool
	if len(toolItems) > 0 {
		if firstTool, ok := toolItems[0].(ToolItem); ok {
			mdl.selectedTool = firstTool.Name
			mdl.loadCommandsForTool(firstTool.Name, commandItems)
		}
	}

	p := tea.NewProgram(mdl)
	_, err = p.Run()
	return err
}
func (m model) Init() tea.Cmd {
	return textarea.Blink
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return &m, tea.Quit
		case "ctrl+e":
			if m.cursorMode == "textarea" {
				cmdStr := m.textarea.Value()
				out, err := runCommand(cmdStr)
				if err != nil {
					m.errorMsg = fmt.Sprintf("⚠️ %v", err)
					m.viewport.SetContent(out)
				} else {
					m.viewport.SetContent(out)
				}
			}
		case "tab":
			switch m.cursorMode {
			case "tools":
				m.cursorMode = "commands"
			case "commands":
				m.cursorMode = "textarea"
			case "textarea":
				m.cursorMode = "viewport"
			case "viewport":
				m.cursorMode = "tools"
			}
		case "shift+tab":
			switch m.cursorMode {
			case "tools":
				m.cursorMode = "viewport"
			case "viewport":
				m.cursorMode = "textarea"
			case "textarea":
				m.cursorMode = "commands"
			case "commands":
				m.cursorMode = "tools"
			}
		case "enter":
			if m.cursorMode == "tools" {
				if selected, ok := m.toolsList.SelectedItem().(ToolItem); ok {
					m.selectedTool = selected.Name
					m.loadCommandsForTool(selected.Name, m.allCommands)
				}
			} else if m.cursorMode == "commands" {
				if selected, ok := m.commandsList.SelectedItem().(CommandItem); ok {
					full := selected.Command.CommandStub + " " + selected.Command.Flags
					m.textarea.SetValue(full)
					m.viewport.SetContent(renderExamples(selected.Examples))
				}
			}
		case "ctrl+y":
			if m.cursorMode == "textarea" {
				_ = clipboard.WriteAll(m.textarea.Value())
				m.yankMessage = "Copied to clipboard!"
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
		case "v":
			if m.cursorMode == "viewport" {
				m.visualMode = !m.visualMode
				if m.visualMode {
					m.visualStart = m.viewport.YOffset
					m.visualEnd = m.visualStart
				}
			}
		case "ctrl+x":
			if m.cursorMode == "textarea" {
				m.textarea.SetValue("")
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	switch m.cursorMode {
	case "commands":
		m.commandsList, cmd = m.commandsList.Update(msg)
		cmds = append(cmds, cmd)
	case "tools":
		m.toolsList, cmd = m.toolsList.Update(msg)
		cmds = append(cmds, cmd)
	case "viewport":
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.yankMessage != "" {
		m.yankMessage = ""
	}

	return &m, tea.Batch(cmds...)
}

func runCommand(input string) (string, error) {
	cmd := exec.Command("sh", "-c", input)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func renderExamples(examples []db.Example) string {
	if len(examples) == 0 {
		return "No examples available."
	}
	var b strings.Builder
	for _, e := range examples {
		b.WriteString("💡 ")
		b.WriteString(e.Example)
		if e.Notes != "" {
			b.WriteString("\n   ✏️ " + e.Notes)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

func (m *model) loadCommandsForTool(tool string, commands []CommandItem) {
	var cmdItems []list.Item
	for _, c := range commands {
		if c.Command.ToolName == tool {
			cmdItems = append(cmdItems, c)
		}
	}
	m.commandsList.SetItems(cmdItems)
	m.commandsList.Title = "Commands for " + tool
}

func (m model) View() string {
	var b strings.Builder

	header := lipgloss.NewStyle().Bold(true).Render("Grimmoire") + "\n\n"
	b.WriteString(header)

	// Layout calculations
	padding := 8
	usableHeight := max(m.height - padding, 4)

	listHeight := usableHeight / 2
	viewportHeight := usableHeight - listHeight

	m.toolsList.SetSize(m.width/2-1, listHeight)
	m.commandsList.SetSize(m.width/2-1, listHeight)
	m.viewport.Width = m.width
	m.viewport.Height = viewportHeight

	// Search Bar (top)
	searchBar := lipgloss.NewStyle().
		Width(m.width).
		MarginBottom(1).
		Render(m.textarea.View())

	// Tool and Command lists side-by-side
	dualPane := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().
			Width(m.width/2-1).
			Border(lipgloss.NormalBorder()).
			Render(m.toolsList.View()),
		lipgloss.NewStyle().
			Width(m.width/2-1).
			Border(lipgloss.NormalBorder()).
			Render(m.commandsList.View()),
	)

	// Viewport content with visual selection if active
	lines := strings.Split(m.viewport.View(), "\n")
	if m.visualMode {
		start := m.visualStart
		end := m.visualEnd
		if start > end {
			start, end = end, start
		}
		for i := start; i <= end && i < len(lines); i++ {
			lines[i] = lipgloss.NewStyle().
				Background(lipgloss.Color("57")).
				Render(lines[i])
		}
	}
	vp := lipgloss.NewStyle().
		Width(m.width).
		Height(viewportHeight).
		Render(strings.Join(lines, "\n"))

	// Footer
	footer := lipgloss.NewStyle().
		MarginTop(1).
		Render("[Tab] Switch Focus | [Enter] Select | [Ctrl+E] Execute | [q] Quit")

	// Assemble
	b.WriteString(searchBar)
	b.WriteString(dualPane + "\n\n")
	b.WriteString(vp)
	b.WriteString("\n" + footer)

	if m.yankMessage != "" {
		b.WriteString("\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Render(m.yankMessage))
	}
	if m.errorMsg != "" {
		b.WriteString("\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Render("Error: "+m.errorMsg))
	}

	return b.String()
}
