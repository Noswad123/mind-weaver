package recipes

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/urfave/cli/v2"
)

type RecipeResult struct {
	ID           string `json:"id"`
	NoteID       string `json:"noteID"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ServingSize  string `json:"servingSize,omitempty"`
	PrepTime     string `json:"prepTime,omitempty"`
	CookingTime  string `json:"cookingTime,omitempty"`
	Meal         string `json:"meal,omitempty"`
	Instructions string `json:"instructions,omitempty"`
	PayloadJSON  string `json:"payloadJSON,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

type IngredientResult struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	IngredientType string `json:"ingredientType,omitempty"`
	Notes          string `json:"notes,omitempty"`
	RecipeCount    int    `json:"recipeCount"`
	MentionCount   int    `json:"mentionCount"`
	CreatedAt      string `json:"createdAt,omitempty"`
	UpdatedAt      string `json:"updatedAt,omitempty"`
}

type IngredientMentionResult struct {
	ID                    string   `json:"id"`
	NoteID                string   `json:"noteID"`
	RecipeID              string   `json:"recipeID"`
	RecipeName            string   `json:"recipeName"`
	Path                  string   `json:"path"`
	RawText               string   `json:"rawText"`
	RawName               string   `json:"rawName"`
	QuantityText          string   `json:"quantityText,omitempty"`
	QuantityNumber        *float64 `json:"quantityNumber,omitempty"`
	UnitRaw               string   `json:"unitRaw,omitempty"`
	CanonicalIngredientID string   `json:"canonicalIngredientID,omitempty"`
	CanonicalName         string   `json:"canonicalName,omitempty"`
	LineNumber            *int64   `json:"lineNumber,omitempty"`
}

func QueryRecipes(c *cli.Context, svc *Service) error {
	rows, err := svc.ListRecipes(c.Context, readScopeDomains(c))
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	out := make([]RecipeResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, recipeResult(row))
	}
	return writeJSON(out)
}

func QueryProjection(c *cli.Context, svc *Service) error {
	projection := strings.ToLower(strings.TrimSpace(c.Args().First()))
	switch projection {
	case "recipe", "recipes":
		return QueryRecipes(c, svc)
	case "":
		return cli.Exit("❌ projection name is required (example: mw query projection recipe --scope cooking)", 1)
	default:
		return cli.Exit(fmt.Sprintf("❌ unsupported projection %q", projection), 1)
	}
}

func QueryIngredients(c *cli.Context, svc *Service) error {
	if c.Bool("mentions") || c.Bool("unresolved") {
		return QueryIngredientMentions(c, svc)
	}

	rows, err := svc.ListIngredients(c.Context)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	out := make([]IngredientResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, IngredientResult{
			ID:             strconv.Itoa(row.ID),
			Name:           row.Name,
			IngredientType: row.IngredientType,
			Notes:          row.Notes,
			RecipeCount:    row.RecipeCount,
			MentionCount:   row.MentionCount,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	return writeJSON(out)
}

func QueryIngredientMentions(c *cli.Context, svc *Service) error {
	rows, err := svc.ListIngredientMentions(c.Context, c.Bool("unresolved"))
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	out := make([]IngredientMentionResult, 0, len(rows))
	for _, row := range rows {
		out = append(out, mentionResult(row))
	}
	return writeJSON(out)
}

func recipeResult(row db.RecipeRow) RecipeResult {
	return RecipeResult{
		ID:           strconv.Itoa(row.ID),
		NoteID:       strconv.Itoa(row.NoteID),
		Name:         row.Name,
		Path:         row.Path,
		ServingSize:  row.ServingSize,
		PrepTime:     row.PrepTime,
		CookingTime:  row.CookingTime,
		Meal:         row.Meal,
		Instructions: row.Instructions,
		PayloadJSON:  row.PayloadJSON,
		UpdatedAt:    row.UpdatedAt,
	}
}

func mentionResult(row db.RecipeIngredientMentionRow) IngredientMentionResult {
	var qty *float64
	if row.QuantityNumber.Valid {
		v := row.QuantityNumber.Float64
		qty = &v
	}
	canonicalID := ""
	if row.CanonicalIngredientID.Valid {
		canonicalID = strconv.FormatInt(row.CanonicalIngredientID.Int64, 10)
	}
	canonicalName := ""
	if row.CanonicalName.Valid {
		canonicalName = row.CanonicalName.String
	}
	var line *int64
	if row.LineNumber.Valid {
		v := row.LineNumber.Int64
		line = &v
	}
	return IngredientMentionResult{
		ID:                    strconv.Itoa(row.ID),
		NoteID:                strconv.Itoa(row.NoteID),
		RecipeID:              strconv.Itoa(row.RecipeID),
		RecipeName:            row.RecipeName,
		Path:                  row.Path,
		RawText:               row.RawText,
		RawName:               row.RawName,
		QuantityText:          row.QuantityText,
		QuantityNumber:        qty,
		UnitRaw:               row.UnitRaw,
		CanonicalIngredientID: canonicalID,
		CanonicalName:         canonicalName,
		LineNumber:            line,
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func readScopeDomains(c *cli.Context) []string {
	values := c.StringSlice("scope")
	args := c.Args().Slice()
	for i := 0; i < len(args); i++ {
		arg := strings.TrimSpace(args[i])
		switch {
		case arg == "--scope" || arg == "--scopes":
			if i+1 < len(args) {
				values = append(values, args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--scope="):
			values = append(values, strings.TrimPrefix(arg, "--scope="))
		case strings.HasPrefix(arg, "--scopes="):
			values = append(values, strings.TrimPrefix(arg, "--scopes="))
		}
	}

	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}
