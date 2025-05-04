package todo

import (
	"regexp"
	"strings"
)

type Todo struct {
	Group        string
	DerivedGroup *string
	Task         *string
	Status       string
	RawStatus    string
	Level        int
	Depth        int
	Line         int
	IsGroup      bool
}

var statusMap = map[string]string{
	"x": "done",
	"=": "hold",
	"?": "ambiguous",
	" ": "todo",
	"-": "pending",
	"+": "recurring",
	"_": "cancelled",
	"!": "important",
}

func ExtractTodos(content string) []Todo {
	lines := strings.Split(content, "\n")
	todos := []Todo{}
	type Group struct {
		Level int
		Name  string
	}
	groupStack := []Group{}

	groupRe := regexp.MustCompile(`^(\*+)\s+(?:\((.)\)\s+)?(.*)$`)
	taskRe := regexp.MustCompile(`^(\s*)(-+)\s+\((.)\)\s+(.*)$`)

	for i, line := range lines {
		lineNum := i + 1

		if match := groupRe.FindStringSubmatch(line); match != nil {
			level := len(match[1])
			rawStatus := match[2]
			if rawStatus == "" {
				rawStatus = " "
			}
			name := strings.TrimSpace(match[3])
			status := statusMap[rawStatus]
			if status == "" {
				status = "todo"
			}

			for len(groupStack) > 0 && groupStack[len(groupStack)-1].Level >= level {
				groupStack = groupStack[:len(groupStack)-1]
			}
			groupStack = append(groupStack, Group{Level: level, Name: name})

			var derived *string
			if len(groupStack) >= 2 {
				parent := groupStack[len(groupStack)-2].Name
				derived = &parent
			}

			todos = append(todos, Todo{
				Group:        name,
				DerivedGroup: derived,
				Task:         &name,
				Status:       status,
				RawStatus:    rawStatus,
				Level:        level,
				Depth:        0,
				Line:         lineNum,
				IsGroup:      true,
			})
			continue
		}

		if match := taskRe.FindStringSubmatch(line); match != nil {
			dashes := match[2]
			rawStatus := match[3]
			taskText := strings.TrimSpace(match[4])
			depth := len(dashes)
			status := statusMap[rawStatus]
			if status == "" {
				status = "todo"
			}

			groupName := ""
			level := 0
			var derived *string
			if len(groupStack) > 0 {
				groupName = groupStack[len(groupStack)-1].Name
				level = groupStack[len(groupStack)-1].Level
			}
			if len(groupStack) >= 2 {
				parent := groupStack[len(groupStack)-2].Name
				derived = &parent
			}

			todos = append(todos, Todo{
				Group:        groupName,
				DerivedGroup: derived,
				Task:         &taskText,
				Status:       status,
				RawStatus:    rawStatus,
				Level:        level,
				Depth:        depth,
				Line:         lineNum,
				IsGroup:      false,
			})
		}
	}
	return todos
}
