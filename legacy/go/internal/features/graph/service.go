package graph

import (
	"context"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/graph/ui"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
)

type Service struct {
	store                 Store
	defaultNeighborsLimit int
	defaultPreviewLines   int
}

type Store interface {
	GetNoteLiteByID(id int) (db.NoteLiteRow, error)
	GetBacklinksByNoteID(noteID, limit, offset int) ([]db.NoteLiteRow, error)
	GetOutlinksByNoteID(noteID, limit, offset int) ([]db.NoteLiteRow, error)
	RecomputeConnectedness() error
	GetNoteContentByID(id int) (string, error)
	ListNotesByConnectedness(limit, offset int) ([]db.NoteDegreeRow, error)
	SearchNotesByConnectedness(q string, limit, offset int) ([]db.NoteDegreeRow, error)
	ListGraphNodes(ctx context.Context) ([]db.GraphNodeRow, error)
	ListGraphEdges(ctx context.Context) ([]db.GraphEdgeRow, error)
}

func New(noteDb *db.NoteDb) *Service {
	return &Service{
		store:                 noteDb,
		defaultNeighborsLimit: 50,
		defaultPreviewLines:   40,
	}
}

func (s *Service) LoadFocus(noteID int, neighborsLimit int) (ui.FocusView, error) {
	if neighborsLimit <= 0 {
		neighborsLimit = s.defaultNeighborsLimit
	}
	focus, err := s.store.GetNoteLiteByID(noteID)
	if err != nil {
		return ui.FocusView{}, err
	}

	backlinks, err := s.store.GetBacklinksByNoteID(noteID, neighborsLimit, 0)
	if err != nil {
		return ui.FocusView{}, err
	}

	outlinks, err := s.store.GetOutlinksByNoteID(noteID, neighborsLimit, 0)
	if err != nil {
		return ui.FocusView{}, err
	}

	content, err := s.store.GetNoteContentByID(noteID)
	if err != nil {
		return ui.FocusView{}, err
	}

	return ui.FocusView{
		Focus:      toGraphNoteLite(focus),
		Backlinks:  toGraphNoteLiteList(backlinks),
		Outlinks:   toGraphNoteLiteList(outlinks),
		Preview:    trimPreview(content, s.defaultPreviewLines),
		Unresolved: 0,
	}, nil
}

func (s *Service) ListIndex(limit, offset int) ([]ui.ConnectednessRow, error) {
	rows, err := s.store.ListNotesByConnectedness(limit, offset)
	if err != nil {
		return nil, err
	}
	return toGraphConnectednessRows(rows), nil
}

func (s *Service) SearchIndex(query string, limit, offset int) ([]ui.ConnectednessRow, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return s.ListIndex(limit, offset)
	}

	rows, err := s.store.SearchNotesByConnectedness(query, limit, offset)
	if err != nil {
		return nil, err
	}
	return toGraphConnectednessRows(rows), nil
}

func (s *Service) RecomputeConnectedness() error {
	return s.store.RecomputeConnectedness()
}

func (s *Service) ResolveStartNote(query string) (*ui.NoteLite, error) {
	query = strings.TrimSpace(query)

	var rows []ui.ConnectednessRow
	var err error

	if query == "" {
		rows, err = s.ListIndex(1, 0)
	} else {
		rows, err = s.SearchIndex(query, 1, 0)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	n := rows[0].Note
	return &n, nil
}

func toGraphConnectednessRows(rows []db.NoteDegreeRow) []ui.ConnectednessRow {
	out := make([]ui.ConnectednessRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ui.ConnectednessRow{
			Note: ui.NoteLite{ID: r.ID, Title: r.Title, Path: r.Path},
			In:   r.In,
			Out:  r.Out,
		})
	}
	return out
}

func toGraphNoteLite(r db.NoteLiteRow) ui.NoteLite {
	return ui.NoteLite{ID: r.ID, Title: r.Title, Path: r.Path}
}

func toGraphNoteLiteList(rows []db.NoteLiteRow) []ui.NoteLite {
	out := make([]ui.NoteLite, 0, len(rows))
	for _, r := range rows {
		out = append(out, toGraphNoteLite(r))
	}
	return out
}

func trimPreview(content string, maxLines int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		lines = append(lines, "…")
	}
	return strings.Join(lines, "\n")
}
