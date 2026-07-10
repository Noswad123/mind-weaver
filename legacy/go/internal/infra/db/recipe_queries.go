package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (db *NoteDb) ListRecipes(ctx context.Context, scopeDomains []string) ([]RecipeRow, error) {
	scopeDomains = normalizeScopeDomains(scopeDomains)

	query := `
		SELECT r.id, r.note_id, r.name, n.path, COALESCE(r.serving_size,''), COALESCE(r.prep_time,''),
		       COALESCE(r.cooking_time,''), COALESCE(r.meal,''), COALESCE(r.instructions,''),
		       COALESCE(r.payload_json,'{}'), COALESCE(r.updated_at,'')
		FROM recipes r
		JOIN notes n ON n.id = r.note_id
	`
	args := make([]any, 0, len(scopeDomains))
	for i, domain := range scopeDomains {
		alias := fmt.Sprintf("scope_%d", i)
		query += fmt.Sprintf("\n\t\tJOIN note_domains %s ON %s.note_id = n.id AND %s.domain = ?", alias, alias, alias)
		args = append(args, domain)
	}
	query += `
		ORDER BY r.name COLLATE NOCASE
	`

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RecipeRow{}
	for rows.Next() {
		var r RecipeRow
		if err := rows.Scan(&r.ID, &r.NoteID, &r.Name, &r.Path, &r.ServingSize, &r.PrepTime, &r.CookingTime, &r.Meal, &r.Instructions, &r.PayloadJSON, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func normalizeScopeDomains(domains []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		out = append(out, domain)
	}
	return out
}

func (db *NoteDb) ListIngredients(ctx context.Context) ([]IngredientRow, error) {
	rows, err := db.conn.QueryContext(ctx, `
		SELECT i.id, i.name, COALESCE(i.ingredient_type,''), COALESCE(i.notes,''),
		       COUNT(DISTINCT rim.recipe_id) AS recipe_count,
		       COUNT(rim.id) AS mention_count,
		       COALESCE(i.created_at,''), COALESCE(i.updated_at,'')
		FROM ingredients i
		LEFT JOIN recipe_ingredient_mentions rim ON rim.canonical_ingredient_id = i.id
		GROUP BY i.id
		ORDER BY i.name COLLATE NOCASE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []IngredientRow{}
	for rows.Next() {
		var r IngredientRow
		if err := rows.Scan(&r.ID, &r.Name, &r.IngredientType, &r.Notes, &r.RecipeCount, &r.MentionCount, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (db *NoteDb) ListRecipeIngredientMentions(ctx context.Context, unresolvedOnly bool) ([]RecipeIngredientMentionRow, error) {
	query := `
		SELECT rim.id, rim.note_id, rim.recipe_id, r.name, n.path, rim.raw_text, rim.raw_name,
		       COALESCE(rim.quantity_text,''), rim.quantity_number, COALESCE(rim.unit_raw,''),
		       rim.canonical_ingredient_id, i.name, rim.line_number
		FROM recipe_ingredient_mentions rim
		JOIN recipes r ON r.id = rim.recipe_id
		JOIN notes n ON n.id = rim.note_id
		LEFT JOIN ingredients i ON i.id = rim.canonical_ingredient_id
	`
	if unresolvedOnly {
		query += ` WHERE rim.canonical_ingredient_id IS NULL`
	}
	query += ` ORDER BY r.name COLLATE NOCASE, rim.line_number ASC, rim.id ASC`

	rows, err := db.conn.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RecipeIngredientMentionRow{}
	for rows.Next() {
		var r RecipeIngredientMentionRow
		if err := rows.Scan(&r.ID, &r.NoteID, &r.RecipeID, &r.RecipeName, &r.Path, &r.RawText, &r.RawName, &r.QuantityText, &r.QuantityNumber, &r.UnitRaw, &r.CanonicalIngredientID, &r.CanonicalName, &r.LineNumber); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
