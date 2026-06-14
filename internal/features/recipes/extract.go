package recipes

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

type RecipeProjection struct {
	Name         string              `json:"name"`
	ServingSize  string              `json:"servingSize,omitempty"`
	PrepTime     string              `json:"prepTime,omitempty"`
	CookingTime  string              `json:"cookingTime,omitempty"`
	Meal         []string            `json:"meal,omitempty"`
	Instructions []string            `json:"instructions,omitempty"`
	Ingredients  []IngredientMention `json:"ingredients,omitempty"`
}

type IngredientMention struct {
	RawText        string   `json:"rawText"`
	RawName        string   `json:"rawName"`
	QuantityText   string   `json:"quantityText,omitempty"`
	QuantityNumber *float64 `json:"quantityNumber,omitempty"`
	UnitRaw        string   `json:"unitRaw,omitempty"`
	LineNumber     int      `json:"lineNumber,omitempty"`
}

var quantityStartRe = regexp.MustCompile(`(?i)^((?:\d+\s+\d+/\d+)|(?:\d+/\d+)|(?:\d+(?:\.\d+)?)|(?:\.\d+))\s*(.*)$`)

var knownUnits = map[string]struct{}{
	"tsp": {}, "teaspoon": {}, "teaspoons": {},
	"tbsp": {}, "tablespoon": {}, "tablespoons": {},
	"cup": {}, "cups": {},
	"oz": {}, "ounce": {}, "ounces": {},
	"lb": {}, "lbs": {}, "pound": {}, "pounds": {},
	"g": {}, "gram": {}, "grams": {},
	"kg": {}, "kilogram": {}, "kilograms": {},
	"ml": {}, "milliliter": {}, "milliliters": {},
	"l": {}, "liter": {}, "liters": {},
	"pinch": {}, "clove": {}, "cloves": {},
	"stalk": {}, "stalks": {}, "can": {}, "cans": {},
	"slice": {}, "slices": {}, "fillet": {}, "fillets": {},
}

func Extract(content, fallbackName string, meta map[string]string) RecipeProjection {
	projection := RecipeProjection{
		Name:        firstNonEmpty(meta["recipe_name"], meta["name"], fallbackName),
		ServingSize: firstNonEmpty(meta["serving_size"], meta["servings"]),
		PrepTime:    firstNonEmpty(meta["prep_time"], meta["prep"]),
		CookingTime: firstNonEmpty(meta["cooking_time"], meta["cook_time"]),
		Meal:        parseList(meta["meal"]),
	}

	lines := strings.Split(content, "\n")
	inFrontmatter := false
	inIngredients := false
	inInstructions := false
	ingredientLevel := 0
	instructionLevel := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter || trimmed == "" {
			continue
		}

		if level, heading, ok := parseHeading(trimmed); ok {
			lower := strings.ToLower(heading)
			if strings.Contains(lower, "ingredient") {
				inIngredients = true
				ingredientLevel = level
				inInstructions = false
				continue
			}
			if isInstructionHeading(lower) {
				inInstructions = true
				instructionLevel = level
				inIngredients = false
				continue
			}
			if inIngredients && level <= ingredientLevel {
				inIngredients = false
			}
			if inInstructions && level <= instructionLevel {
				inInstructions = false
			}
			continue
		}

		if inIngredients {
			if text, ok := parseBullet(trimmed); ok {
				mention := ParseIngredientMention(text)
				mention.LineNumber = i + 1
				if mention.RawName != "" {
					projection.Ingredients = append(projection.Ingredients, mention)
				}
			}
			continue
		}

		if inInstructions {
			if text, ok := parseBulletOrOrdered(trimmed); ok {
				projection.Instructions = append(projection.Instructions, text)
			}
		}
	}

	return projection
}

func ParseIngredientMention(raw string) IngredientMention {
	raw = strings.TrimSpace(raw)
	mention := IngredientMention{RawText: raw, RawName: normalizeIngredientName(raw)}

	match := quantityStartRe.FindStringSubmatch(raw)
	if len(match) == 0 {
		return mention
	}

	mention.QuantityText = strings.TrimSpace(match[1])
	if n, ok := parseQuantity(mention.QuantityText); ok {
		mention.QuantityNumber = &n
	}

	rest := strings.TrimSpace(match[2])
	parts := strings.Fields(rest)
	if len(parts) > 0 {
		unit := strings.Trim(strings.ToLower(parts[0]), ".,;:")
		if _, ok := knownUnits[unit]; ok {
			mention.UnitRaw = parts[0]
			rest = strings.TrimSpace(strings.TrimPrefix(rest, parts[0]))
		}
	}
	mention.RawName = normalizeIngredientName(rest)
	return mention
}

func PayloadJSON(p RecipeProjection) string {
	b, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseHeading(trimmed string) (int, string, bool) {
	level := 0
	for _, r := range trimmed {
		if r == '#' {
			level++
			continue
		}
		break
	}
	if level == 0 || level > 6 || len(trimmed) <= level || trimmed[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(trimmed[level:]), true
}

func parseBullet(trimmed string) (string, bool) {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return strings.TrimSpace(trimmed[2:]), true
	}
	return "", false
}

func parseBulletOrOrdered(trimmed string) (string, bool) {
	if text, ok := parseBullet(trimmed); ok {
		return text, true
	}
	idx := strings.Index(trimmed, ".")
	if idx > 0 {
		if _, err := strconv.Atoi(trimmed[:idx]); err == nil {
			text := strings.TrimSpace(trimmed[idx+1:])
			return text, text != ""
		}
	}
	return "", false
}

func isInstructionHeading(lower string) bool {
	return strings.Contains(lower, "instruction") || strings.Contains(lower, "method") || strings.Contains(lower, "direction") || strings.Contains(lower, "preparation")
}

func normalizeIngredientName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "("); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	s = strings.Trim(s, " .,:;-")
	return s
}

func parseQuantity(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	parts := strings.Fields(s)
	if len(parts) == 2 && strings.Contains(parts[1], "/") {
		whole, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, false
		}
		frac, ok := parseFraction(parts[1])
		return whole + frac, ok
	}
	if strings.Contains(s, "/") {
		return parseFraction(s)
	}
	n, err := strconv.ParseFloat(s, 64)
	return n, err == nil
}

func parseFraction(s string) (float64, bool) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return 0, false
	}
	n, nerr := strconv.ParseFloat(parts[0], 64)
	d, derr := strconv.ParseFloat(parts[1], 64)
	if nerr != nil || derr != nil || d == 0 {
		return 0, false
	}
	return n / d, true
}

func parseList(s string) []string {
	s = strings.TrimSpace(strings.Trim(s, "[]"))
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(strings.Trim(part, "\"'"))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
