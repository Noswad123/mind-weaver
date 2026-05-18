package registry

import "github.com/Noswad123/mind-weaver/internal/core/shared"

type Entry struct {
	NoteID    shared.ID
	UID       string
	Path      string
	IsHub     bool
	UpdatedAt string
}

type Conflict struct {
	NoteID     *shared.ID
	UID        *string
	Path       string
	IsHub      bool
	Reason     string
	DetectedAt string
}
