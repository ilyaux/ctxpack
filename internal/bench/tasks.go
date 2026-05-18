package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ParseTasksFile(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return parseJSONTasks(data)
	}
	return parseYAMLTasks(string(data))
}

func parseJSONTasks(data []byte) ([]Task, error) {
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err == nil {
		return tasks, nil
	}
	var wrapper struct {
		Tasks []Task `json:"tasks"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Tasks, nil
}

func parseYAMLTasks(text string) ([]Task, error) {
	var tasks []Task
	var current *Task
	section := ""
	for lineNo, raw := range strings.Split(text, "\n") {
		line := stripComment(raw)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "tasks:" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if strings.Contains(item, ":") {
				if current != nil {
					tasks = append(tasks, *current)
				}
				current = &Task{}
				section = ""
				if err := applyTaskField(current, item, lineNo+1); err != nil {
					return nil, err
				}
				continue
			}
			if current == nil {
				return nil, fmt.Errorf("line %d: list item before task", lineNo+1)
			}
			value := strings.Trim(item, `"'`)
			switch section {
			case "expect":
				current.Expect = append(current.Expect, value)
			case "avoid":
				current.Avoid = append(current.Avoid, value)
			default:
				return nil, fmt.Errorf("line %d: list item outside expect/avoid", lineNo+1)
			}
			continue
		}
		if current == nil {
			return nil, fmt.Errorf("line %d: task field before task item", lineNo+1)
		}
		if strings.HasSuffix(trimmed, ":") {
			section = strings.TrimSuffix(trimmed, ":")
			continue
		}
		section = ""
		if err := applyTaskField(current, trimmed, lineNo+1); err != nil {
			return nil, err
		}
	}
	if current != nil {
		tasks = append(tasks, *current)
	}
	return tasks, nil
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func applyTaskField(task *Task, field string, lineNo int) error {
	key, value, ok := strings.Cut(field, ":")
	if !ok {
		return fmt.Errorf("line %d: expected key: value", lineNo)
	}
	key = strings.TrimSpace(key)
	value = strings.Trim(strings.TrimSpace(value), `"'`)
	switch key {
	case "name":
		task.Name = value
	case "task":
		task.Task = value
	case "budget":
		budget, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("line %d: invalid budget %q", lineNo, value)
		}
		task.Budget = budget
	default:
		return fmt.Errorf("line %d: unsupported task key %q", lineNo, key)
	}
	return nil
}
