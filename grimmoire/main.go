package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	sections   []string
	cursor     int
	command    string
	commandOut string
}

func initialModel() model {
	cmd := exec.Command("man", "grep")
	out, _ := cmd.Output()
	sections := parseManSections(string(out))
	return model{
		sections:   sections,
		command:    "grep",
		commandOut: "",
	}
}

func parseManSections(text string) []string {
	// Very rough split on common section headings
	var sections []string
	lines := strings.Split(text, "\n")
	var currentSection string
	for _, line := range lines {
		if strings.TrimSpace(line) == strings.ToUpper(line) && len(line) > 0 && len(line) < 30 {
			if currentSection != "" {
				sections = append(sections, currentSection)
			}
			currentSection = fmt.Sprintf("# %s\n", line)
		} else {
			currentSection += line + "\n"
		}
	}
	if currentSection != "" {
		sections = append(sections, currentSection)
	}
	return sections
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sections)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	s := "Use ↑ ↓ to navigate, q to quit.\n\n"
	for i, section := range m.sections {
		title := strings.Split(section, "\n")[0]
		if i == m.cursor {
			s += fmt.Sprintf("> %s\n", title)
		} else {
			s += fmt.Sprintf("  %s\n", title)
		}
	}
	s += "\n" + strings.Repeat("-", 40) + "\n"
	s += m.sections[m.cursor]
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
