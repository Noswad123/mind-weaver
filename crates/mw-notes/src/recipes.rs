use regex::Regex;

#[derive(Debug, Clone, Default, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RecipeProjection {
    pub name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub serving_size: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub prep_time: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub cooking_time: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub meal: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub instructions: Vec<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub ingredients: Vec<IngredientMention>,
}

#[derive(Debug, Clone, Default, PartialEq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct IngredientMention {
    pub raw_text: String,
    pub raw_name: String,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub quantity_text: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub quantity_number: Option<f64>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub unit_raw: String,
    #[serde(skip_serializing_if = "is_zero")]
    pub line_number: usize,
}

fn is_zero(value: &usize) -> bool {
    *value == 0
}

pub fn extract_recipe_projection(
    content: &str,
    fallback_name: &str,
    meta: &std::collections::BTreeMap<String, String>,
) -> RecipeProjection {
    let mut projection = RecipeProjection {
        name: first_non_empty(
            [meta.get("recipe_name"), meta.get("name")]
                .into_iter()
                .flatten(),
            fallback_name,
        ),
        serving_size: first_non_empty(
            [meta.get("serving_size"), meta.get("servings")]
                .into_iter()
                .flatten(),
            "",
        ),
        prep_time: first_non_empty(
            [meta.get("prep_time"), meta.get("prep")]
                .into_iter()
                .flatten(),
            "",
        ),
        cooking_time: first_non_empty(
            [meta.get("cooking_time"), meta.get("cook_time")]
                .into_iter()
                .flatten(),
            "",
        ),
        meal: meta.get("meal").map(|s| parse_list(s)).unwrap_or_default(),
        ..RecipeProjection::default()
    };

    let mut in_frontmatter = false;
    let mut in_ingredients = false;
    let mut in_instructions = false;
    let mut ingredient_level = 0usize;
    let mut instruction_level = 0usize;

    for (idx, line) in content.split('\n').enumerate() {
        let trimmed = line.trim();
        if trimmed == "---" {
            in_frontmatter = !in_frontmatter;
            continue;
        }
        if in_frontmatter || trimmed.is_empty() {
            continue;
        }

        if let Some((level, heading)) = parse_heading(trimmed) {
            let lower = heading.to_ascii_lowercase();
            if lower.contains("ingredient") {
                in_ingredients = true;
                ingredient_level = level;
                in_instructions = false;
                continue;
            }
            if is_instruction_heading(&lower) {
                in_instructions = true;
                instruction_level = level;
                in_ingredients = false;
                continue;
            }
            if in_ingredients && level <= ingredient_level {
                in_ingredients = false;
            }
            if in_instructions && level <= instruction_level {
                in_instructions = false;
            }
            continue;
        }

        if in_ingredients {
            if let Some(text) = parse_bullet(trimmed) {
                let mut mention = parse_ingredient_mention(&text);
                mention.line_number = idx + 1;
                if !mention.raw_name.is_empty() {
                    projection.ingredients.push(mention);
                }
            }
            continue;
        }

        if in_instructions {
            if let Some(text) = parse_bullet_or_ordered(trimmed) {
                projection.instructions.push(text);
            }
        }
    }

    projection
}

pub fn parse_ingredient_mention(raw: &str) -> IngredientMention {
    let raw = raw.trim();
    let mut mention = IngredientMention {
        raw_text: raw.to_string(),
        raw_name: normalize_ingredient_name(raw),
        ..IngredientMention::default()
    };

    let re =
        Regex::new(r"(?i)^((?:\d+\s+\d+/\d+)|(?:\d+/\d+)|(?:\d+(?:\.\d+)?)|(?:\.\d+))\s*(.*)$")
            .unwrap();
    let Some(captures) = re.captures(raw) else {
        return mention;
    };

    mention.quantity_text = captures
        .get(1)
        .map(|m| m.as_str().trim().to_string())
        .unwrap_or_default();
    mention.quantity_number = parse_quantity(&mention.quantity_text);
    let mut rest = captures
        .get(2)
        .map(|m| m.as_str().trim().to_string())
        .unwrap_or_default();
    if let Some(first) = rest.split_whitespace().next() {
        let unit = first
            .trim_matches(['.', ',', ';', ':'])
            .to_ascii_lowercase();
        if is_known_unit(&unit) {
            mention.unit_raw = first.to_string();
            rest = rest[first.len()..].trim().to_string();
        }
    }
    mention.raw_name = normalize_ingredient_name(&rest);
    mention
}

fn parse_heading(trimmed: &str) -> Option<(usize, String)> {
    let level = trimmed.chars().take_while(|ch| *ch == '#').count();
    if level == 0 || level > 6 || trimmed.as_bytes().get(level).copied() != Some(b' ') {
        return None;
    }
    Some((level, trimmed[level..].trim().to_string()))
}

fn parse_bullet(trimmed: &str) -> Option<String> {
    trimmed
        .strip_prefix("- ")
        .or_else(|| trimmed.strip_prefix("* "))
        .map(|s| s.trim().to_string())
        .filter(|s| !s.is_empty())
}

fn parse_bullet_or_ordered(trimmed: &str) -> Option<String> {
    if let Some(text) = parse_bullet(trimmed) {
        return Some(text);
    }
    let (number, rest) = trimmed.split_once('.')?;
    number.trim().parse::<usize>().ok()?;
    let text = rest.trim().to_string();
    (!text.is_empty()).then_some(text)
}

fn is_instruction_heading(lower: &str) -> bool {
    lower.contains("instruction")
        || lower.contains("method")
        || lower.contains("direction")
        || lower.contains("preparation")
}

fn normalize_ingredient_name(value: &str) -> String {
    let before_paren = value.split_once('(').map(|(s, _)| s).unwrap_or(value);
    before_paren
        .trim()
        .trim_matches([' ', '.', ',', ':', ';', '-'])
        .to_string()
}

fn parse_quantity(value: &str) -> Option<f64> {
    let value = value.trim();
    let parts: Vec<&str> = value.split_whitespace().collect();
    if parts.len() == 2 && parts[1].contains('/') {
        return Some(parts[0].parse::<f64>().ok()? + parse_fraction(parts[1])?);
    }
    if value.contains('/') {
        return parse_fraction(value);
    }
    value.parse::<f64>().ok()
}

fn parse_fraction(value: &str) -> Option<f64> {
    let (num, den) = value.split_once('/')?;
    let num = num.parse::<f64>().ok()?;
    let den = den.parse::<f64>().ok()?;
    (den != 0.0).then_some(num / den)
}

fn parse_list(value: &str) -> Vec<String> {
    value
        .trim()
        .trim_matches(['[', ']'])
        .split(',')
        .map(|part| part.trim().trim_matches(['"', '\'']).to_string())
        .filter(|part| !part.is_empty())
        .collect()
}

fn first_non_empty<'a>(values: impl Iterator<Item = &'a String>, fallback: &str) -> String {
    values
        .map(|s| s.trim())
        .find(|s| !s.is_empty())
        .unwrap_or(fallback.trim())
        .to_string()
}

fn is_known_unit(unit: &str) -> bool {
    matches!(
        unit,
        "tsp"
            | "teaspoon"
            | "teaspoons"
            | "tbsp"
            | "tablespoon"
            | "tablespoons"
            | "cup"
            | "cups"
            | "oz"
            | "ounce"
            | "ounces"
            | "lb"
            | "lbs"
            | "pound"
            | "pounds"
            | "g"
            | "gram"
            | "grams"
            | "kg"
            | "kilogram"
            | "kilograms"
            | "ml"
            | "milliliter"
            | "milliliters"
            | "l"
            | "liter"
            | "liters"
            | "pinch"
            | "clove"
            | "cloves"
            | "stalk"
            | "stalks"
            | "can"
            | "cans"
            | "slice"
            | "slices"
            | "fillet"
            | "fillets"
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::extract_metadata;

    #[test]
    fn extracts_recipe_projection() {
        let content = r#"---
domains: [recipe]
servings: 4
meal: [dinner, lunch]
---

# Soup

## Ingredients
- 1 1/2 cups lentils
- salt

## Instructions
1. Rinse lentils
- Simmer
"#;
        let meta = extract_metadata(content);
        let recipe = extract_recipe_projection(content, "Soup", &meta.raw);

        assert_eq!(recipe.name, "Soup");
        assert_eq!(recipe.serving_size, "4");
        assert_eq!(recipe.meal, vec!["dinner", "lunch"]);
        assert_eq!(recipe.ingredients.len(), 2);
        assert_eq!(recipe.ingredients[0].raw_name, "lentils");
        assert_eq!(recipe.ingredients[0].quantity_number, Some(1.5));
        assert_eq!(recipe.ingredients[0].unit_raw, "cups");
        assert_eq!(recipe.instructions, vec!["Rinse lentils", "Simmer"]);
    }
}
