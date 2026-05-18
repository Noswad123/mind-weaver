package notes

import (
	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

func mapNotes(rows []db.NoteRow) []note.Note {
	out := make([]note.Note, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapNote(r))
	}
	return out
}

func mapNote(r db.NoteRow) note.Note {
	links := make([]note.Link, 0, len(r.Links))
	for _, l := range r.Links {
		links = append(links, note.Link{
			Type:         l.Type,
			Target:       l.Target,
			Label:        l.Label,
			ResolvedPath: l.ResolvedPath,
		})
	}

	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}

	return note.Note{
		ID:        r.ID,
		Path:      r.Path,
		Title:     r.Title,
		Tags:      tags,
		Links:     links,
		Content:   r.Content,
		UpdatedAt: r.UpdatedAt,
	}
}


func toLinkRows(links []note.Link) []db.LinkRow {
	out := make([]db.LinkRow, 0, len(links))
	for _, l := range links {
		resolved := ""
		if l.ResolvedPath != "" {
			resolved = l.ResolvedPath
		}
		out = append(out, db.LinkRow{
			Label:        l.Label,
			Target:       l.Target,
			Type:         l.Type,
			ResolvedPath: resolved,
		})
	}
	return out
}
