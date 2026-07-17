package config

import (
	"fmt"
	"strconv"
	"strings"
)

type yamlLine struct {
	indent int
	text   string
	line   int
}

// parseYAML implements the intentionally small YAML subset used by WikiForge's
// generated configuration: indentation maps, block lists, inline lists,
// quoted/unquoted scalars, booleans, integers, nulls, and comments.
func parseYAML(input string) (any, error) {
	var lines []yamlLine
	for i, raw := range strings.Split(strings.ReplaceAll(input, "\t", "    "), "\n") {
		clean := stripYAMLComment(raw)
		if strings.TrimSpace(clean) == "" {
			continue
		}
		indent := len(clean) - len(strings.TrimLeft(clean, " "))
		lines = append(lines, yamlLine{indent: indent, text: strings.TrimSpace(clean), line: i + 1})
	}
	if len(lines) == 0 {
		return map[string]any{}, nil
	}
	value, next, err := parseYAMLBlock(lines, 0, lines[0].indent)
	if err != nil {
		return nil, err
	}
	if next != len(lines) {
		return nil, fmt.Errorf("line %d: unexpected trailing content", lines[next].line)
	}
	return value, nil
}

func parseYAMLBlock(lines []yamlLine, index, indent int) (any, int, error) {
	if index >= len(lines) {
		return map[string]any{}, index, nil
	}
	if lines[index].indent != indent {
		return nil, index, fmt.Errorf("line %d: unexpected indentation", lines[index].line)
	}
	if strings.HasPrefix(lines[index].text, "-") {
		return parseYAMLList(lines, index, indent)
	}
	return parseYAMLMap(lines, index, indent)
}

func parseYAMLMap(lines []yamlLine, index, indent int) (any, int, error) {
	result := map[string]any{}
	for index < len(lines) {
		line := lines[index]
		if line.indent < indent {
			break
		}
		if line.indent > indent {
			return nil, index, fmt.Errorf("line %d: unexpected indentation", line.line)
		}
		if strings.HasPrefix(line.text, "-") {
			break
		}
		key, raw, ok := splitYAMLKeyValue(line.text)
		if !ok || key == "" {
			return nil, index, fmt.Errorf("line %d: expected key: value", line.line)
		}
		index++
		if raw != "" {
			result[key] = parseYAMLScalar(raw)
			continue
		}
		if index < len(lines) && lines[index].indent > indent {
			child, next, err := parseYAMLBlock(lines, index, lines[index].indent)
			if err != nil {
				return nil, index, err
			}
			result[key] = child
			index = next
		} else {
			result[key] = map[string]any{}
		}
	}
	return result, index, nil
}

func parseYAMLList(lines []yamlLine, index, indent int) (any, int, error) {
	result := []any{}
	for index < len(lines) {
		line := lines[index]
		if line.indent < indent {
			break
		}
		if line.indent != indent || !strings.HasPrefix(line.text, "-") {
			break
		}
		item := strings.TrimSpace(strings.TrimPrefix(line.text, "-"))
		index++
		if item == "" {
			if index < len(lines) && lines[index].indent > indent {
				child, next, err := parseYAMLBlock(lines, index, lines[index].indent)
				if err != nil {
					return nil, index, err
				}
				result = append(result, child)
				index = next
			} else {
				result = append(result, nil)
			}
			continue
		}
		if key, raw, ok := splitYAMLKeyValue(item); ok {
			m := map[string]any{}
			if raw != "" {
				m[key] = parseYAMLScalar(raw)
			} else if index < len(lines) && lines[index].indent > indent {
				child, next, err := parseYAMLBlock(lines, index, lines[index].indent)
				if err != nil {
					return nil, index, err
				}
				m[key] = child
				index = next
			} else {
				m[key] = map[string]any{}
			}
			// Remaining indented lines belong to this map item.
			if index < len(lines) && lines[index].indent > indent {
				moreAny, next, err := parseYAMLMap(lines, index, lines[index].indent)
				if err != nil {
					return nil, index, err
				}
				for k, v := range moreAny.(map[string]any) {
					m[k] = v
				}
				index = next
			}
			result = append(result, m)
			continue
		}
		result = append(result, parseYAMLScalar(item))
	}
	return result, index, nil
}

func splitYAMLKeyValue(s string) (string, string, bool) {
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ':':
			if !inSingle && !inDouble {
				return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+1:]), true
			}
		}
	}
	return "", "", false
}

func stripYAMLComment(s string) string {
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return strings.TrimRight(s[:i], " ")
			}
		}
	}
	return strings.TrimRight(s, " ")
}

func parseYAMLScalar(s string) any {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if inner == "" {
			return []any{}
		}
		parts := splitYAMLInlineList(inner)
		out := make([]any, 0, len(parts))
		for _, p := range parts {
			out = append(out, parseYAMLScalar(p))
		}
		return out
	}
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	case "null", "~":
		return nil
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return s
}

func splitYAMLInlineList(s string) []string {
	var parts []string
	start := 0
	inSingle, inDouble := false, false
	for i, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ',':
			if !inSingle && !inDouble {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}
