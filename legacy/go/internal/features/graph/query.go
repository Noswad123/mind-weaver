package graph

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/urfave/cli/v2"
)

type QueryOptions struct {
	Search string
	Domain string
	Depth  int
	Limit  int
}

type GraphResult struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	Meta  GraphMeta   `json:"meta"`
}

type GraphMeta struct {
	SeedCount int    `json:"seedCount"`
	Depth     int    `json:"depth"`
	Search    string `json:"search"`
	Domain    string `json:"domain"`
}

type GraphNode struct {
	ID      string   `json:"id"`
	NoteID  int      `json:"noteID"`
	UID     string   `json:"uid"`
	Label   string   `json:"label"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`
	Tags    []string `json:"tags"`
	Domains []string `json:"domains"`
	Matched bool     `json:"matched"`
	Unknown bool     `json:"unknown"`
}

type GraphEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Label  string `json:"label"`
}

func QueryGraph(c *cli.Context, svc *Service) error {
	result, err := svc.QueryGraph(c.Context, QueryOptions{
		Search: c.String("search"),
		Domain: c.String("domain"),
		Depth:  c.Int("depth"),
		Limit:  c.Int("limit"),
	})
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func (s *Service) QueryGraph(ctx context.Context, opts QueryOptions) (GraphResult, error) {
	if opts.Depth < 0 {
		opts.Depth = 0
	}
	if opts.Limit <= 0 {
		opts.Limit = 250
	}

	nodeRows, err := s.store.ListGraphNodes(ctx)
	if err != nil {
		return GraphResult{}, err
	}
	edgeRows, err := s.store.ListGraphEdges(ctx)
	if err != nil {
		return GraphResult{}, err
	}

	nodesByID := map[int]db.GraphNodeRow{}
	for _, node := range nodeRows {
		nodesByID[node.ID] = node
	}

	seed := map[int]struct{}{}
	search := strings.ToLower(strings.TrimSpace(opts.Search))
	domain := strings.ToLower(strings.TrimSpace(opts.Domain))
	if search == "" && domain == "" {
		for _, node := range nodeRows {
			seed[node.ID] = struct{}{}
			if len(seed) >= opts.Limit {
				break
			}
		}
	} else {
		for _, node := range nodeRows {
			if graphNodeMatches(node, search, domain) {
				seed[node.ID] = struct{}{}
				if len(seed) >= opts.Limit {
					break
				}
			}
		}
	}

	included := expandGraphNodes(seed, edgeRows, opts.Depth, opts.Limit)
	if len(included) == 0 {
		return GraphResult{Nodes: []GraphNode{}, Edges: []GraphEdge{}, Meta: GraphMeta{SeedCount: len(seed), Depth: opts.Depth, Search: opts.Search, Domain: opts.Domain}}, nil
	}

	nodes := []GraphNode{}
	for id := range included {
		row, ok := nodesByID[id]
		if !ok {
			continue
		}
		_, matched := seed[id]
		nodes = append(nodes, GraphNode{
			ID:      graphNodeID(id),
			NoteID:  id,
			UID:     row.UID,
			Label:   graphNodeLabel(row),
			Title:   row.Title,
			Path:    row.Path,
			Kind:    "note",
			Tags:    row.Tags,
			Domains: row.Domains,
			Matched: matched,
			Unknown: strings.TrimSpace(row.UID) == "",
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return strings.ToLower(nodes[i].Label) < strings.ToLower(nodes[j].Label) })

	edges := []GraphEdge{}
	seenEdges := map[string]struct{}{}
	for _, edge := range edgeRows {
		if _, ok := included[edge.SourceID]; !ok {
			continue
		}
		if _, ok := included[edge.TargetID]; !ok {
			continue
		}
		id := graphNodeID(edge.SourceID) + "->" + graphNodeID(edge.TargetID) + ":" + edge.Label
		if _, ok := seenEdges[id]; ok {
			continue
		}
		seenEdges[id] = struct{}{}
		edges = append(edges, GraphEdge{ID: id, Source: graphNodeID(edge.SourceID), Target: graphNodeID(edge.TargetID), Kind: "mentions", Label: edge.Label})
	}

	return GraphResult{Nodes: nodes, Edges: edges, Meta: GraphMeta{SeedCount: len(seed), Depth: opts.Depth, Search: opts.Search, Domain: opts.Domain}}, nil
}

func graphNodeMatches(node db.GraphNodeRow, search, domain string) bool {
	if domain != "" {
		found := false
		for _, d := range node.Domains {
			if strings.EqualFold(strings.TrimSpace(d), domain) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if search == "" {
		return true
	}
	haystack := strings.ToLower(node.UID + " " + node.Title + " " + node.Path + " " + strings.Join(node.Tags, " ") + " " + strings.Join(node.Domains, " "))
	return strings.Contains(haystack, search)
}

func expandGraphNodes(seed map[int]struct{}, edges []db.GraphEdgeRow, depth, limit int) map[int]struct{} {
	included := map[int]struct{}{}
	frontier := []int{}
	for id := range seed {
		included[id] = struct{}{}
		frontier = append(frontier, id)
	}
	sort.Ints(frontier)

	neighbors := map[int][]int{}
	for _, edge := range edges {
		neighbors[edge.SourceID] = append(neighbors[edge.SourceID], edge.TargetID)
		neighbors[edge.TargetID] = append(neighbors[edge.TargetID], edge.SourceID)
	}

	for hop := 0; hop < depth && len(included) < limit; hop++ {
		next := []int{}
		for _, id := range frontier {
			ns := neighbors[id]
			sort.Ints(ns)
			for _, n := range ns {
				if _, ok := included[n]; ok {
					continue
				}
				included[n] = struct{}{}
				next = append(next, n)
				if len(included) >= limit {
					break
				}
			}
			if len(included) >= limit {
				break
			}
		}
		frontier = next
		if len(frontier) == 0 {
			break
		}
	}
	return included
}

func graphNodeID(id int) string { return "note:" + strconv.Itoa(id) }

func graphNodeLabel(row db.GraphNodeRow) string {
	if strings.TrimSpace(row.UID) != "" {
		return row.UID
	}
	return "unknown"
}
