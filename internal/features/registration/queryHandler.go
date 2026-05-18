package registration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/Noswad123/mind-weaver/internal/core/registry"
)

type RegistryService interface {
	GetNoteIDRegistry() ([]NoteID, error)
	GetNoteIDConflicts() ([]NoteIDConflict, error)
}

type NoteID struct {
	NoteID    int    `json:"note_id"`
	UID       string `json:"uid"`
	Path      string `json:"path"`
	IsHub     bool   `json:"is_hub"`
	UpdatedAt string `json:"updated_at"`
}

type NoteIDConflict struct {
	NoteID     *int    `json:"note_id,omitempty"`
	UID        *string `json:"uid,omitempty"`
	Path       string  `json:"path"`
	IsHub      bool    `json:"is_hub"`
	Reason     string  `json:"reason"`
	DetectedAt string  `json:"detected_at"`
}

type registryQueryResult struct {
	NoteIDs   []registry.Entry    `json:"note_ids"`
	Conflicts []registry.Conflict `json:"conflicts"`
}

func QueryRegistry(c *cli.Context, r registry.Reader) error {
	ctx := context.Background()

	ids, err := r.ListEntries(ctx)
	if err != nil {
		return cli.Exit("❌ Failed to fetch note_ids: "+err.Error(), 1)
	}

	conflicts, err := r.ListConflicts(ctx)
	if err != nil {
		return cli.Exit("❌ Failed to fetch note_id_conflicts: "+err.Error(), 1)
	}

	out := registryQueryResult{NoteIDs: ids, Conflicts: conflicts}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return cli.Exit("❌ Failed to encode JSON: "+err.Error(), 1)
	}

	fmt.Fprintln(c.App.Writer, string(b))
	return nil
}
