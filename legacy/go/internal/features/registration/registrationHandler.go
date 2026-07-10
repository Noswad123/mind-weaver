package registration

import (
	"context"
	"log"
	"path/filepath"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
	"github.com/Noswad123/mind-weaver/internal/core/registry"
	"github.com/urfave/cli/v2"
)

func RegisterNotes(c *cli.Context, notesDir string, svc registry.Updater) error {
	ctx := context.Background()
	root := filepath.Clean(notesDir)

	log.Println("🪪 Registering note IDs...")

	res, err := Build(root)
	if err != nil {
		return cli.Exit("❌ Failed to build registry: "+err.Error(), 1)
	}

	entries := make([]registry.Entry, 0, len(res.Registry.Entries))
	conflicts := make([]registry.Conflict, 0, len(res.Duplicates)+len(res.MissingHub))

	// 1) Missing folder note id in the metadata
	for _, rel := range res.MissingHub {
		conflicts = append(conflicts, registry.Conflict{
			NoteID:     nil,
			UID:        nil, // unknown
			Path:       rel,
			IsHub:      true,
			Reason:     "MISSING_HUB_ID",
			DetectedAt: "", // optional; svc/db can fill timestamp if you want
		})
	}

	for _, dup := range res.Duplicates {
		uid := dup.ID
		for _, rel := range dup.Paths {
			isHub := notefiles.IsHubNotePath(rel)
			conflicts = append(conflicts, registry.Conflict{
				NoteID:     nil,  // fill later if exists in DB
				UID:        &uid, // duplicate id from markdown metadata
				Path:       rel,
				IsHub:      isHub,
				Reason:     "DUPLICATE_ID",
				DetectedAt: "",
			})
		}
	}

	// 3) Unique registry entries → try to map to DB note_id
	// res.Registry.Entries only contains NON-duplicates by construction
	for _, e := range res.Registry.Entries {
		nid, err := svc.NoteIDByPath(ctx, e.Path)
		if err != nil {
			return cli.Exit("❌ Failed to resolve note id for "+e.Path+": "+err.Error(), 1)
		}

		isHub := notefiles.IsHubNotePath(e.Path)

		if nid == nil {
			// exists on disk, but not in DB
			uid := e.ID
			conflicts = append(conflicts, registry.Conflict{
				NoteID:     nil,
				UID:        &uid,
				Path:       e.Path,
				IsHub:      isHub,
				Reason:     "NOTE_NOT_IN_DB",
				DetectedAt: "",
			})
			continue
		}

		entries = append(entries, registry.Entry{
			NoteID:    *nid,
			UID:       e.ID,
			Path:      e.Path,
			IsHub:     isHub,
			UpdatedAt: "", // optional; svc/db can fill if needed
		})
	}

	// 4) Fill NoteID for conflicts where possible
	for i := range conflicts {
		if conflicts[i].NoteID != nil {
			continue
		}
		nid, err := svc.NoteIDByPath(ctx, conflicts[i].Path)
		if err == nil && nid != nil {
			conflicts[i].NoteID = nid
		}
	}

	if err := svc.ReplaceRegistry(ctx, entries, conflicts); err != nil {
		return cli.Exit("❌ Failed to write registry to DB: "+err.Error(), 1)
	}

	log.Printf("✅ Registry updated: %d registered, %d conflict(s)\n", len(entries), len(conflicts))
	return nil
}
