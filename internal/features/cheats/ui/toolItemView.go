package ui

type ToolItem struct {
	Name string
}

func (t ToolItem) Title() string       { return t.Name }
func (t ToolItem) Description() string { return "" }
func (t ToolItem) FilterValue() string { return t.Name }
