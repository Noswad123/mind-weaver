package db

import (
	"database/sql"
	"strings"
	"time"
)

type RecipeProjectionWrite struct {
	Name         string
	ServingSize  string
	PrepTime     string
	CookingTime  string
	Meal         string
	Instructions string
	PayloadJSON  string
	Ingredients  []RecipeIngredientMentionWrite
}

type RecipeIngredientMentionWrite struct {
	RawText        string
	RawName        string
	QuantityText   string
	QuantityNumber *float64
	UnitRaw        string
	LineNumber     int
}

func (t *Tx) ClearRecipeProjection(noteID int) error {
	if _, err := t.tx.Exec(`DELETE FROM recipe_ingredient_mentions WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	if _, err := t.tx.Exec(`DELETE FROM recipes WHERE note_id = ?`, noteID); err != nil {
		return err
	}
	return nil
}

func (t *Tx) UpsertRecipeProjection(noteID int, p RecipeProjectionWrite) error {
	if err := t.ClearRecipeProjection(noteID); err != nil {
		return err
	}

	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = "Untitled recipe"
	}

	res, err := t.tx.Exec(`
		INSERT INTO recipes (note_id, name, serving_size, prep_time, cooking_time, meal, instructions, payload_json, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, noteID, name, p.ServingSize, p.PrepTime, p.CookingTime, p.Meal, p.Instructions, p.PayloadJSON, time.Now().Format(time.RFC3339))
	if err != nil {
		return err
	}
	recipeID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for _, ingredient := range p.Ingredients {
		rawName := strings.TrimSpace(ingredient.RawName)
		if rawName == "" {
			continue
		}

		ingredientID, err := t.upsertIngredient(rawName)
		if err != nil {
			return err
		}

		var qty sql.NullFloat64
		if ingredient.QuantityNumber != nil {
			qty.Valid = true
			qty.Float64 = *ingredient.QuantityNumber
		}

		_, err = t.tx.Exec(`
			INSERT INTO recipe_ingredient_mentions (
				note_id, recipe_id, raw_text, raw_name, quantity_text, quantity_number, unit_raw, canonical_ingredient_id, line_number
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, noteID, recipeID, ingredient.RawText, rawName, ingredient.QuantityText, qty, ingredient.UnitRaw, ingredientID, ingredient.LineNumber)
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *Tx) upsertIngredient(name string) (int64, error) {
	name = strings.TrimSpace(name)
	if _, err := t.tx.Exec(`
		INSERT INTO ingredients (name, updated_at)
		VALUES (?, ?)
		ON CONFLICT(name) DO UPDATE SET updated_at=excluded.updated_at
	`, name, time.Now().Format(time.RFC3339)); err != nil {
		return 0, err
	}

	var id int64
	if err := t.tx.QueryRow(`SELECT id FROM ingredients WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}
