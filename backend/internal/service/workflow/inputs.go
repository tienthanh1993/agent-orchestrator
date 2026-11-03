package workflow

import (
	"strings"

	"github.com/aoagents/agent-orchestrator/backend/internal/httpd/apierr"
)

type placeholder struct {
	name       string
	start, end int
}

// Parse only the template. Submitted values are never interpreted as templates.
func placeholders(task string) ([]placeholder, error) {
	var fields []placeholder
	for offset := 0; offset < len(task); {
		i := strings.Index(task[offset:], "{{")
		if i < 0 {
			break
		}
		start := offset + i
		j := strings.Index(task[start+2:], "}}")
		if j < 0 {
			return nil, apierr.Invalid("INVALID_WORKFLOW_TEMPLATE", "Close each input placeholder with }}", nil)
		}
		end := start + 2 + j
		name := strings.TrimSpace(task[start+2 : end])
		if name == "" || strings.ContainsAny(name, "{}\r\n") {
			return nil, apierr.Invalid("INVALID_WORKFLOW_TEMPLATE", "Input placeholders need a nonempty name on one line, without braces", nil)
		}
		fields = append(fields, placeholder{name: name, start: start, end: end + 2})
		offset = end + 2
	}
	return fields, nil
}

func resolveInputs(task string, defaults, submitted map[string]string) (map[string]string, string, error) {
	fields, err := placeholders(task)
	if err != nil {
		return nil, "", err
	}
	values := make(map[string]string)
	problems := make(map[string]string)
	for _, f := range fields {
		value := defaults[f.name]
		if override, ok := submitted[f.name]; ok {
			value = override
		}
		values[f.name] = value
		if strings.TrimSpace(value) == "" {
			problems[f.name] = "Enter a value"
		}
	}
	for name := range submitted {
		if _, ok := values[name]; !ok {
			problems[name] = "Unknown input"
		}
	}
	if len(problems) > 0 {
		return values, "", apierr.Invalid("INVALID_WORKFLOW_INPUTS", "Fill in every workflow input", map[string]any{"fields": problems})
	}
	var resolved strings.Builder
	offset := 0
	for _, f := range fields {
		if resolved.Len()+f.start-offset+len(values[f.name]) > 32000 {
			return values, "", apierr.Invalid("INVALID_WORKFLOW_INPUTS", "Filled-in master prompt exceeds 32000 bytes", map[string]any{"fields": map[string]string{f.name: "Shorten this value"}})
		}
		resolved.WriteString(task[offset:f.start])
		resolved.WriteString(values[f.name])
		offset = f.end
	}
	resolved.WriteString(task[offset:])
	if resolved.Len() > 32000 {
		return values, "", apierr.Invalid("INVALID_WORKFLOW_INPUTS", "Filled-in master prompt exceeds 32000 bytes", nil)
	}
	return values, resolved.String(), nil
}
