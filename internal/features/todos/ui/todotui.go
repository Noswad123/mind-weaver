package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	theme "github.com/Noswad123/mind-weaver/internal/shared/ui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	todoPrefixRe     = regexp.MustCompile(`^(?:-|\*|##+)?\s*\[.*?\]\s*`)
	sourceSuffixRe   = regexp.MustCompile(`\s+\[\[([^\]]+)\]\]\s*$`)
	trailingWeightRe = regexp.MustCompile(`\s*w:[0-9]+(?:\.[0-9]+)?\s*$`)
	multiSpaceMetaRe = regexp.MustCompile(`\s{2,}`)
)

type model struct {
	groups             []string
	activeTab          int
	todoMap            map[string][]parser.Todo
	cursor             int
	filePath           string
	inboxPath          string
	verbose            bool
	sourceUpdated      bool
	sourcePathByID     map[string]string
	sourceDefaultsByID map[string]sourceDefaults
	archiveMarks       map[string]bool
	editMode           bool
	editBuffer         string
	editMessage        string
	addMode            bool
	addBuffer          string
}

func (m model) Init() tea.Cmd { return nil }

func RunTodoTUI(filePath string, notesDir string, inboxPath string, parsedTodos map[string][]parser.Todo, targetGroups []string) (bool, []string, error) {
	groups := append([]string{"All"}, targetGroups...)

	sourcePathByID, defaultsByID, err := loadTaskIndexSourceContext(notesDir)
	if err != nil {
		return false, nil, err
	}

	initialModel := model{
		groups:             groups,
		todoMap:            parsedTodos,
		filePath:           filePath,
		inboxPath:          inboxPath,
		sourcePathByID:     sourcePathByID,
		sourceDefaultsByID: defaultsByID,
		archiveMarks:       map[string]bool{},
	}

	p := tea.NewProgram(initialModel)
	finalModel, err := p.Run()
	if err != nil {
		return false, nil, err
	}

	if m, ok := finalModel.(model); ok {
		changed, err := m.SaveToFile()
		if err != nil {
			return false, nil, err
		}
		return changed, m.selectedArchiveKeys(), nil
	}
	return false, nil, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.editMode {
			return m.updateEditMode(msg)
		}
		if m.addMode {
			return m.updateAddMode(msg)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(m.groups)
			m.cursor = 0
			m.editMessage = ""
		case "shift+tab", "left", "h":
			m.activeTab--
			if m.activeTab < 0 {
				m.activeTab = len(m.groups) - 1
			}
			m.cursor = 0
			m.editMessage = ""
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.groups[m.activeTab] != "All" && m.cursor < len(m.currentList())-1 {
				m.cursor++
			}
		case " ":
			m.toggleCurrent()
		case "a":
			m.toggleArchiveCurrent()
		case "v":
			m.verbose = !m.verbose
		case "e":
			m.startEditCurrent()
		case "i":
			m.startAdd()
		}
	}
	return m, nil
}

func (m model) updateEditMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.editMessage = "Edit canceled"
		return m, nil
	case "enter":
		if err := m.saveMetadataForCurrent(); err != nil {
			m.editMessage = fmt.Sprintf("Edit failed: %v", err)
			return m, nil
		}
		m.editMode = false
		m.editMessage = "Metadata updated"
		return m, nil
	case "backspace", "ctrl+h":
		runes := []rune(m.editBuffer)
		if len(runes) > 0 {
			m.editBuffer = string(runes[:len(runes)-1])
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.editBuffer += string(msg.Runes)
		}
		return m, nil
	}
}

func (m *model) startEditCurrent() {
	if m.groups[m.activeTab] == "All" {
		m.editMessage = "Select a category task to edit"
		return
	}
	list := m.currentList()
	if len(list) == 0 || m.cursor < 0 || m.cursor >= len(list) {
		return
	}

	meta := strings.TrimSpace(list[m.cursor].MetaSummary)
	meta = strings.TrimSpace(trailingWeightRe.ReplaceAllString(meta, ""))
	meta = multiSpaceMetaRe.ReplaceAllString(meta, " ")

	m.editMode = true
	m.editBuffer = meta
	m.editMessage = ""
}

func (m *model) saveMetadataForCurrent() error {
	group := m.groups[m.activeTab]
	if group == "All" {
		return fmt.Errorf("editing is only available in category tabs")
	}
	list := m.currentList()
	if len(list) == 0 || m.cursor < 0 || m.cursor >= len(list) {
		return fmt.Errorf("no task selected")
	}

	current := list[m.cursor]
	sourceID, taskText, ok := splitDashboardTaskSourceInfo(current.Text)
	if !ok {
		return fmt.Errorf("selected task is missing source marker")
	}

	sourcePath, ok := m.sourcePathByID[sourceID]
	if !ok {
		return fmt.Errorf("could not resolve source note for %s", sourceID)
	}

	occurrence := m.sourceTaskOccurrence(sourceID, taskText, current.Line)
	metadata := strings.TrimSpace(m.editBuffer)
	if err := upsertTaskMetadataInSource(sourcePath, taskText, occurrence, metadata); err != nil {
		return err
	}

	defaults := m.sourceDefaultsByID[sourceID]
	if defaults.Priority == "" {
		defaults.Priority = "p3"
	}
	if defaults.Energy == "" {
		defaults.Energy = "medium"
	}

	combined := taskText
	if metadata != "" {
		combined = combined + " " + metadata
	}
	weight := parser.DeriveTodoWeightWithDefaults(combined, defaults.Priority, defaults.Energy)

	summary := fmt.Sprintf("w:%.2f", weight)
	if metadata != "" {
		summary = metadata + "  " + summary
	}

	current.Weight = weight
	current.MetaSummary = summary
	list[m.cursor] = current
	m.todoMap[group] = list
	m.sourceUpdated = true

	return nil
}

func (m *model) startAdd() {
	m.addMode = true
	m.addBuffer = ""
	m.editMessage = ""
}

func (m model) updateAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.addMode = false
		m.editMessage = "Add canceled"
		return m, nil
	case "enter":
		if err := m.saveToInbox(); err != nil {
			m.editMessage = fmt.Sprintf("Add failed: %v", err)
			return m, nil
		}
		m.addMode = false
		m.editMessage = "Todo added to inbox"
		return m, nil
	case "backspace", "ctrl+h":
		runes := []rune(m.addBuffer)
		if len(runes) > 0 {
			m.addBuffer = string(runes[:len(runes)-1])
		}
		return m, nil
	default:
		if len(msg.Runes) > 0 {
			m.addBuffer += string(msg.Runes)
		}
		return m, nil
	}
}

func (m *model) saveToInbox() error {
	if m.addBuffer == "" {
		return nil
	}

	inboxPath := strings.TrimSpace(m.inboxPath)
	if inboxPath == "" {
		return fmt.Errorf("inbox path is not configured")
	}
	if err := os.MkdirAll(filepath.Dir(inboxPath), 0o755); err != nil {
		return fmt.Errorf("failed to create inbox directory: %w", err)
	}

	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open inbox file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("\n- [ ] %s", m.addBuffer)); err != nil {
		return fmt.Errorf("failed to write to inbox file: %w", err)
	}

	return nil
}

func (m *model) toggleArchiveCurrent() {
	group := m.groups[m.activeTab]
	if group == "All" {
		m.editMessage = "Select a category task to mark for archive"
		return
	}

	list := m.currentList()
	if len(list) == 0 || m.cursor < 0 || m.cursor >= len(list) {
		return
	}

	key := taskMarkerKey(group, list[m.cursor].Line)
	if m.archiveMarks[key] {
		delete(m.archiveMarks, key)
		m.editMessage = "Archive mark removed"
		return
	}

	m.archiveMarks[key] = true
	list[m.cursor].IsDone = true
	m.todoMap[group] = list
	m.editMessage = "Task will be archived on exit"
}

func (m model) sourceTaskOccurrence(sourceID, taskText string, currentLine int) int {
	count := 0
	for _, group := range m.groups {
		if group == "All" {
			continue
		}
		for _, t := range m.todoMap[group] {
			sid, txt, ok := splitDashboardTaskSourceInfo(t.Text)
			if !ok {
				continue
			}
			if sid == sourceID && txt == taskText && t.Line <= currentLine {
				count++
			}
		}
	}
	if count <= 0 {
		return 1
	}
	return count
}

func (m model) selectedArchiveKeys() []string {
	keys := []string{}
	seen := map[string]struct{}{}

	for _, group := range m.groups {
		if group == "All" {
			continue
		}
		for _, t := range m.todoMap[group] {
			if !m.isArchiveMarked(group, t.Line) {
				continue
			}
			sourceID, taskText, ok := splitDashboardTaskSourceInfo(t.Text)
			if !ok {
				continue
			}
			occurrence := m.sourceTaskOccurrence(sourceID, taskText, t.Line)
			key := archiveSelectionKey(sourceID, taskText, occurrence)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	return keys
}

func archiveSelectionKey(sourceID, taskText string, occurrence int) string {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(taskText)), " ")
	return strings.TrimSpace(sourceID) + "\x1f" + normalized + "\x1f" + strconv.Itoa(occurrence)
}

func taskMarkerKey(group string, line int) string {
	return group + "#" + strconv.Itoa(line)
}

func (m model) isArchiveMarked(group string, line int) bool {
	return m.archiveMarks[taskMarkerKey(group, line)]
}

func (m model) View() string {
	var doc strings.Builder
	doc.WriteString("\n")

	var renderedTabs []string
	for i, cat := range m.groups {
		style := theme.TabStyle
		if i == m.activeTab {
			style = theme.ActiveTabStyle
		}
		renderedTabs = append(renderedTabs, style.Render(cat))
	}
	doc.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...) + "\n\n")

	if m.groups[m.activeTab] == "All" {
		doc.WriteString(m.renderGlobalDashboard())
	} else {
		doc.WriteString(m.renderCategoryList())
	}

	if m.editMode {
		doc.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.Subtle).Render("Edit metadata (e.g. p2 e:l due:2026-03-25 start:2026-03-20 w:2.0)") + "\n")
		doc.WriteString(lipgloss.NewStyle().Foreground(theme.Accent).Render("> "+m.editBuffer) + "\n")
		doc.WriteString(lipgloss.NewStyle().Foreground(theme.Subtle).Render("Enter: Save • Esc: Cancel • Backspace: Delete"))
	} else if m.addMode {
		doc.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.Subtle).Render("Add new todo to inbox") + "\n")
		doc.WriteString(lipgloss.NewStyle().Foreground(theme.Accent).Render("> "+m.addBuffer) + "\n")
		doc.WriteString(lipgloss.NewStyle().Foreground(theme.Subtle).Render("Enter: Save • Esc: Cancel • Backspace: Delete"))
	} else {
		doc.WriteString("\n\n" + lipgloss.NewStyle().Foreground(theme.Subtle).Render("↑/↓: Navigate • Space: Toggle • a: Mark Archive • e: Edit Meta • i: Inbox • v: Verbose • Tab: Switch View • q: Save & Quit\n"))
	}

	if strings.TrimSpace(m.editMessage) != "" {
		doc.WriteString("\n" + lipgloss.NewStyle().Foreground(theme.Subtle).Render(m.editMessage))
	}

	return doc.String()
}

func (m model) currentList() []parser.Todo {
	return m.todoMap[m.groups[m.activeTab]]
}

func (m *model) toggleCurrent() {
	groupName := m.groups[m.activeTab]
	if groupName == "All" {
		return
	}

	list := m.todoMap[groupName]
	if len(list) == 0 {
		return
	}

	list[m.cursor].IsDone = !list[m.cursor].IsDone
	m.todoMap[groupName] = list
}

func (m model) renderCategoryList() string {
	list := m.currentList()
	if len(list) == 0 {
		return lipgloss.NewStyle().Italic(true).Foreground(theme.Subtle).Render("No tasks in this category.")
	}

	var s strings.Builder
	for i, t := range list {
		cursor := "  "
		if m.cursor == i {
			cursor = theme.CursorStyle.Render("> ")
		}

		checkbox := "[ ]"
		if t.IsDone {
			checkbox = "[x]"
		}

		text := sourceSuffixRe.ReplaceAllString(todoPrefixRe.ReplaceAllString(t.Text, ""), "")

		textStyle := lipgloss.NewStyle()
		if t.IsDone {
			textStyle = textStyle.Faint(true).Strikethrough(true)
		}

		archiveBadge := ""
		if m.isArchiveMarked(m.groups[m.activeTab], t.Line) {
			archiveBadge = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(" [A]")
		}

		s.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, checkbox, textStyle.Render(text), archiveBadge))
		if m.verbose {
			meta := strings.TrimSpace(t.MetaSummary)
			if meta == "" {
				meta = fmt.Sprintf("w:%.2f", t.Weight)
			}
			s.WriteString(lipgloss.NewStyle().Foreground(theme.Subtle).Render("    "+meta) + "\n")
		}
	}
	return s.String()
}

func (m model) renderGlobalDashboard() string {
	var s strings.Builder
	for _, group := range m.groups {
		if group == "All" {
			continue
		}

		todos := m.todoMap[group]
		if len(todos) == 0 {
			s.WriteString(fmt.Sprintf("%s %s\n", theme.LabelStyle.Render(group), theme.AlertStyle.Render()))
			continue
		}

		progressTasks := make([]theme.ProgressTask, 0, len(todos))
		for _, t := range todos {
			progressTasks = append(progressTasks, theme.ProgressTask{Completed: t.IsDone, Weight: t.Weight})
		}
		s.WriteString(theme.RenderWeightedProgressBar(group, progressTasks) + "\n")
	}
	return s.String()
}

func (m model) SaveToFile() (bool, error) {
	content, err := os.ReadFile(m.filePath)
	if err != nil {
		return false, err
	}

	lines := strings.Split(string(content), "\n")
	checkboxRe := regexp.MustCompile(`\[( |x|X|-)\]`)

	for _, group := range m.groups {
		if group == "All" {
			continue
		}
		for _, t := range m.todoMap[group] {
			lineIdx := t.Line - 1
			if lineIdx < 0 || lineIdx >= len(lines) {
				continue
			}

			line := lines[lineIdx]
			newBox := "[ ]"
			if t.IsDone {
				newBox = "[x]"
			}

			loc := checkboxRe.FindStringIndex(line)
			if loc != nil {
				lines[lineIdx] = line[:loc[0]] + newBox + line[loc[1]:]
			}
		}
	}

	updated := strings.Join(lines, "\n")
	if updated == string(content) {
		return m.sourceUpdated, nil
	}

	if err := os.WriteFile(m.filePath, []byte(updated), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
