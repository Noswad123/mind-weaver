package parser

type Tool struct {
	ID          int
	Name        string
	Description string
}

type Cheat struct {
	ID            int
	ToolID        int
	Section       string
	Context       string
	CommandStub   string
	Flags         string
	Description   string
	OptionalInfo  string
}



type Example struct {
	ID        int
	CheatID   int
	Example   string
	Notes     string
}

type ToolYAML struct {
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Cheats      []CheatYAML `yaml:"cheats"`
}

type CheatYAML struct {
	Section      string        `yaml:"section"`
	Context      string        `yaml:"context"`
	CommandStub  string        `yaml:"command_stub"`
	Flags        string        `yaml:"flags"`
	Description  string        `yaml:"description"`
	OptionalInfo string        `yaml:"optional_info"`
	Tags         []string      `yaml:"tags"`
	Args         []ArgYAML     `yaml:"args"`
	Examples     []ExampleYAML `yaml:"examples"`
}

type ArgYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type ExampleYAML struct {
	Example string `yaml:"example"`
	Notes   string `yaml:"notes"`
}
