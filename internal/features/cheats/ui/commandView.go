package ui

type Cheat struct {
	ID           int
	CommandStub  string
	Flags        string
	Description  string
	OptionalInfo string
}

func (c Cheat) Title() string       { return c.CommandStub }
func (c Cheat) Desc() string { return c.Description }
func (c Cheat) FilterValue() string { return c.CommandStub }
