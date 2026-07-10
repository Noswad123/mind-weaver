package note

import (
	"github.com/Noswad123/mind-weaver/internal/core/shared"
)

type Lite struct {
	ID    shared.ID
	Title string
	Path  string
}

type Note struct {
	ID        shared.ID
	UID       string // optional; service fills when resolving by UID
	Path      string
	Title     string
	Content   string
	Tags      []string
	Domains   []string
	Links     []Link
	UpdatedAt string
}

type Document struct {
	ID      shared.ID
	Path    string
	Content string
}
