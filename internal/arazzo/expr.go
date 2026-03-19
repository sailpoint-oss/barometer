package arazzo

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// RuntimeContext holds values available during workflow execution ($inputs, $steps, $response, $sourceDescriptions).
type RuntimeContext struct {
	Inputs           map[string]any            // workflow inputs
	Steps            map[string]StepOutputs    // stepId -> outputs
	Response         *StepResponse             // current step response (statusCode, header, body)
	SourceDescriptions map[string]any          // name -> e.g. { url: "..." }
}

// StepOutputs is the outputs map for a step.
type StepOutputs map[string]any

// StepResponse is the HTTP response for the current step.
type StepResponse struct {
	StatusCode int
	Header     map[string]string
	Body       any // parsed JSON or raw
}

// Resolve evaluates an expression (e.g. $inputs.username, $response.body) and returns the value.
// If value is not a string starting with $, it is returned as-is.
func (c *RuntimeContext) Resolve(value any) (any, error) {
	if c == nil {
		return value, nil
	}
	s, ok := value.(string)
	if !ok {
		return value, nil
	}
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "$") {
		return value, nil
	}
	path := strings.TrimPrefix(s, "$")
	return c.getPath(path)
}

func (c *RuntimeContext) getPath(path string) (any, error) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty expression path")
	}
	switch parts[0] {
	case "inputs":
		return getValuePath(c.Inputs, parts[1:])
	case "steps":
		if len(parts) < 2 {
			return nil, fmt.Errorf("steps requires stepId")
		}
		stepOut, ok := c.Steps[parts[1]]
		if !ok {
			return nil, fmt.Errorf("step %q not found", parts[1])
		}
		if len(parts) == 2 {
			return stepOut, nil
		}
		if len(parts) < 4 || parts[2] != "outputs" {
			return nil, fmt.Errorf("steps.stepId.outputs.field required")
		}
		return getValuePath(stepOut, parts[3:])
	case "response":
		if c.Response == nil {
			return nil, fmt.Errorf("no response in context")
		}
		if len(parts) == 1 {
			return c.Response, nil
		}
		switch parts[1] {
		case "body":
			if len(parts) == 2 {
				return c.Response.Body, nil
			}
			// $response.body.token → path into Body with ["token"]; $response.body[0].id → ["0","id"]
			return getValuePath(c.Response.Body, parts[2:])
		case "header":
			if len(parts) < 3 {
				return c.Response.Header, nil
			}
			// parts[2] is header name (e.g. X-Expires-After)
			if v, ok := c.Response.Header[parts[2]]; ok {
				return v, nil
			}
			// case-insensitive lookup
			for k, v := range c.Response.Header {
				if strings.EqualFold(k, parts[2]) {
					return v, nil
				}
			}
			return nil, nil
		case "statusCode":
			return c.Response.StatusCode, nil
		default:
			return nil, fmt.Errorf("response: unknown field %q", parts[1])
		}
	case "statusCode":
		if c.Response != nil {
			return c.Response.StatusCode, nil
		}
		return nil, fmt.Errorf("no response for statusCode")
	case "sourceDescriptions":
		if len(parts) < 2 {
			return c.SourceDescriptions, nil
		}
		src, ok := c.SourceDescriptions[parts[1]]
		if !ok {
			return nil, fmt.Errorf("sourceDescription %q not found", parts[1])
		}
		if len(parts) == 2 {
			return src, nil
		}
		return getValuePath(toMap(src), parts[2:])
	default:
		return nil, fmt.Errorf("unknown expression root %q", parts[0])
	}
}

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]any)
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

// getValuePath descends into value using parts; supports maps and slices (numeric index).
func getValuePath(value any, parts []string) (any, error) {
	if len(parts) == 0 {
		return value, nil
	}
	switch v := value.(type) {
	case map[string]any:
		next, ok := v[parts[0]]
		if !ok {
			return nil, nil
		}
		return getValuePath(next, parts[1:])
	case StepOutputs:
		next, ok := v[parts[0]]
		if !ok {
			return nil, nil
		}
		return getValuePath(next, parts[1:])
	case map[interface{}]interface{}:
		next, ok := v[parts[0]]
		if !ok {
			return nil, nil
		}
		return getValuePath(next, parts[1:])
	case []interface{}:
		idx, err := strconv.Atoi(parts[0])
		if err != nil || idx < 0 || idx >= len(v) {
			return nil, nil
		}
		return getValuePath(v[idx], parts[1:])
	default:
		return value, nil
	}
}

func (c *RuntimeContext) getMapPath(m map[string]any, parts []string) (any, error) {
	if m == nil {
		return nil, nil
	}
	if len(parts) == 0 {
		return m, nil
	}
	v, ok := m[parts[0]]
	if !ok {
		return nil, nil
	}
	if len(parts) == 1 {
		return v, nil
	}
	return getValuePath(v, parts[1:])
}

// splitPath splits $inputs.foo.bar into ["inputs","foo","bar"]; handles brackets for array index.
func splitPath(path string) []string {
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		case '[':
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
			j := strings.IndexByte(path[i:], ']')
			if j < 0 {
				cur.WriteByte(path[i])
				continue
			}
			parts = append(parts, path[i+1:i+j])
			i += j
		default:
			cur.WriteByte(path[i])
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// ResolveString returns a string value for the expression (for headers, query params).
func (c *RuntimeContext) ResolveString(value any) (string, error) {
	v, err := c.Resolve(value)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", nil
	}
	switch x := v.(type) {
	case string:
		return x, nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	default:
		b, _ := json.Marshal(x)
		return string(b), nil
	}
}

var simpleConditionRegex = regexp.MustCompile(`^\s*(\$[^\s!=<>]+)\s*(==|!=|<=|>=|<|>)\s*(.+)\s*$`)

// EvalSimpleCondition evaluates a simple condition like "$statusCode == 200".
// String comparison is case-insensitive per spec.
func (c *RuntimeContext) EvalSimpleCondition(condition string) (bool, error) {
	condition = strings.TrimSpace(condition)
	// Try to parse left op right
	subs := simpleConditionRegex.FindStringSubmatch(condition)
	if len(subs) != 4 {
		return false, fmt.Errorf("invalid simple condition: %q", condition)
	}
	leftExpr := strings.TrimSpace(subs[1])
	op := strings.TrimSpace(subs[2])
	rightLiteral := strings.TrimSpace(subs[3])

	left, err := c.getPath(strings.TrimPrefix(leftExpr, "$"))
	if err != nil {
		return false, err
	}
	right := parseLiteral(rightLiteral)
	return compare(left, right, op)
}

func parseLiteral(s string) any {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'")
	}
	if s == "null" {
		return nil
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

func compare(left, right any, op string) (bool, error) {
	switch op {
	case "==":
		return eq(left, right), nil
	case "!=":
		return !eq(left, right), nil
	}
	// numeric or string comparison for <, <=, >, >=
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if lok && rok {
		switch op {
		case "<":
			return lf < rf, nil
		case "<=":
			return lf <= rf, nil
		case ">":
			return lf > rf, nil
		case ">=":
			return lf >= rf, nil
		}
	}
	ls, rs := fmt.Sprint(left), fmt.Sprint(right)
	switch op {
	case "<":
		return strings.ToLower(ls) < strings.ToLower(rs), nil
	case "<=":
		return strings.ToLower(ls) <= strings.ToLower(rs), nil
	case ">":
		return strings.ToLower(ls) > strings.ToLower(rs), nil
	case ">=":
		return strings.ToLower(ls) >= strings.ToLower(rs), nil
	}
	return false, fmt.Errorf("unknown operator %q", op)
}

func eq(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// numeric
	af, aok := toFloat(a)
	bf, bok := toFloat(b)
	if aok && bok {
		return af == bf
	}
	// string comparison case-insensitive per spec
	return strings.EqualFold(fmt.Sprint(a), fmt.Sprint(b))
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	default:
		return 0, false
	}
}
