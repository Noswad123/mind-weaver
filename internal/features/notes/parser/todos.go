package parser

import "strings"

func ExtractTodosBySection(content string, targetCategories []string) map[string][]Todo {
	lines := strings.Split(content, "\n")
	results := make(map[string][]Todo)
	currentCategory := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "#") {
			cleanHeading := strings.TrimLeft(trimmed, "# ")
			currentCategory = ""
			for _, cat := range targetCategories {
				if cleanHeading == cat {
					currentCategory = cat
					break
				}
			}
			continue
		}

		if currentCategory != "" && (strings.Contains(trimmed, "[ ]") || strings.Contains(trimmed, "[x]")) {
			isDone := strings.Contains(trimmed, "[x]")
			results[currentCategory] = append(results[currentCategory], Todo{
				Text:   trimmed,
				IsDone: isDone,
				Weight: DeriveTodoWeight(trimmed),
			})
		}
	}
	return results
}
