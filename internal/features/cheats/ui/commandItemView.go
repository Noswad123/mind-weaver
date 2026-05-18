package ui

import (
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type CommandItem struct {
	Command  db.Command
	Examples []db.Example
}

func (c CommandItem) Title() string       { return c.Command.CommandStub }
func (c CommandItem) Description() string { return c.Command.Description }
func (c CommandItem) FilterValue() string { return c.Command.CommandStub }
