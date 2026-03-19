package arazzo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/oliveagle/jsonpath"
)

// EvalCriterion evaluates a single criterion (simple, regex, or jsonpath) and returns true if it passes.
func (c *RuntimeContext) EvalCriterion(crit Criterion) (bool, error) {
	condType := crit.Type
	if condType == "" {
		condType = "simple"
	}
	switch condType {
	case "simple":
		return c.EvalSimpleCondition(crit.Condition)
	case "regex":
		return c.evalRegex(crit)
	case "jsonpath":
		return c.evalJSONPath(crit)
	default:
		return false, fmt.Errorf("unsupported criterion type %q", condType)
	}
}

// EvalSuccessCriteria evaluates all successCriteria; all must pass.
func (c *RuntimeContext) EvalSuccessCriteria(criteria []Criterion) (bool, error) {
	for i, crit := range criteria {
		ok, err := c.EvalCriterion(crit)
		if err != nil {
			return false, fmt.Errorf("criterion %d: %w", i, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

func (c *RuntimeContext) evalRegex(crit Criterion) (bool, error) {
	ctxVal, err := c.Resolve(crit.Context)
	if err != nil {
		return false, err
	}
	str := fmt.Sprint(ctxVal)
	matched, err := regexp.MatchString(crit.Condition, str)
	if err != nil {
		return false, fmt.Errorf("regex: %w", err)
	}
	return matched, nil
}

func (c *RuntimeContext) evalJSONPath(crit Criterion) (bool, error) {
	ctxVal, err := c.Resolve(crit.Context)
	if err != nil {
		return false, err
	}
	// jsonpath library expects a map; ensure we have one
	var data map[string]any
	switch v := ctxVal.(type) {
	case map[string]any:
		data = v
	case map[interface{}]interface{}:
		data = make(map[string]any)
		for k, val := range v {
			if s, ok := k.(string); ok {
				data[s] = val
			}
		}
	default:
		return false, fmt.Errorf("jsonpath context must be object, got %T", ctxVal)
	}
	res, err := jsonpath.JsonPathLookup(data, crit.Condition)
	if err != nil {
		return false, err
	}
	// Criterion passes if the expression returns a truthy value (or non-nil)
	return truthy(res), nil
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return strings.TrimSpace(x) != ""
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	default:
		return true
	}
}
