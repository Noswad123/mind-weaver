package graph

import (
	tui "github.com/Noswad123/mind-weaver/internal/features/graph/ui"
	"github.com/urfave/cli/v2"
)

func ViewNotesGraph(c *cli.Context, graphsvc tui.GraphService) error {
	tui.Run(graphsvc, "")
	return nil
}
