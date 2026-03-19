package arazzo

import (
	"testing"
)

func TestResolve_ArrayIndexing(t *testing.T) {
	ctx := &RuntimeContext{
		Response: &StepResponse{
			StatusCode: 200,
			Body: []interface{}{
				map[string]any{"id": float64(1), "name": "A"},
				map[string]any{"id": float64(2), "name": "B"},
			},
		},
	}
	v, err := ctx.Resolve("$response.body[0].id")
	if err != nil {
		t.Fatal(err)
	}
	if v != float64(1) {
		t.Errorf("got %v want 1", v)
	}
	v, err = ctx.Resolve("$response.body[1].name")
	if err != nil {
		t.Fatal(err)
	}
	if v != "B" {
		t.Errorf("got %v want B", v)
	}
}

func TestResolve_StepsOutputsArrayIndexing(t *testing.T) {
	ctx := &RuntimeContext{
		Steps: map[string]StepOutputs{
			"list": {"items": []interface{}{
				map[string]any{"id": float64(42)},
			}},
		},
	}
	v, err := ctx.Resolve("$steps.list.outputs.items[0].id")
	if err != nil {
		t.Fatal(err)
	}
	if v != float64(42) {
		t.Errorf("got %v want 42", v)
	}
}

func TestResolveString_Number(t *testing.T) {
	ctx := &RuntimeContext{
		Steps: map[string]StepOutputs{
			"list": {"firstId": float64(99)},
		},
	}
	s, err := ctx.ResolveString("$steps.list.outputs.firstId")
	if err != nil {
		t.Fatal(err)
	}
	if s != "99" {
		t.Errorf("got %q want 99", s)
	}
}

func TestEvalSimpleCondition_StatusCode(t *testing.T) {
	ctx := &RuntimeContext{
		Response: &StepResponse{StatusCode: 200},
	}
	ok, err := ctx.EvalSimpleCondition("$statusCode == 200")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
	ok, err = ctx.EvalSimpleCondition("$statusCode == 404")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("expected false")
	}
}

func TestEvalSuccessCriteria_EmptyAndSimple(t *testing.T) {
	ctx := &RuntimeContext{Response: &StepResponse{StatusCode: 200}}
	ok, err := ctx.EvalSuccessCriteria(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true for nil criteria")
	}
	ok, err = ctx.EvalSuccessCriteria([]Criterion{{Condition: "$statusCode == 200"}})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvalCriterion_Regex(t *testing.T) {
	ctx := &RuntimeContext{
		Response: &StepResponse{StatusCode: 200},
	}
	ok, err := ctx.EvalCriterion(Criterion{
		Context:   "$statusCode",
		Condition: `^200$`,
		Type:      "regex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true")
	}
}

func TestEvalCriterion_JSONPath(t *testing.T) {
	ctx := &RuntimeContext{
		Response: &StepResponse{
			Body: map[string]any{
				"items": []interface{}{map[string]any{"id": float64(1)}},
			},
		},
	}
	ok, err := ctx.EvalCriterion(Criterion{
		Context:   "$response.body",
		Condition: "$.items[0].id",
		Type:      "jsonpath",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected true (jsonpath returns first item id)")
	}
}

func TestResolve_Inputs(t *testing.T) {
	ctx := &RuntimeContext{
		Inputs: map[string]any{"username": "alice", "depth": map[string]any{"nested": "val"}},
	}
	v, err := ctx.Resolve("$inputs.username")
	if err != nil {
		t.Fatal(err)
	}
	if v != "alice" {
		t.Errorf("got %v", v)
	}
	v, err = ctx.Resolve("$inputs.depth.nested")
	if err != nil {
		t.Fatal(err)
	}
	if v != "val" {
		t.Errorf("got %v", v)
	}
}

func TestResolve_ResponseBodyToken(t *testing.T) {
	// Simulates login response: body is {"token": "Bearer xxx"} (top-level token)
	ctx := &RuntimeContext{
		Response: &StepResponse{
			StatusCode: 200,
			Body:       map[string]any{"token": "Bearer test-token-alice"},
		},
	}
	v, err := ctx.Resolve("$response.body.token")
	if err != nil {
		t.Fatal(err)
	}
	if v != "Bearer test-token-alice" {
		t.Errorf("got %v want Bearer test-token-alice", v)
	}
}

func TestResolve_NonExpressionReturnsAsIs(t *testing.T) {
	ctx := &RuntimeContext{}
	v, err := ctx.Resolve("literal")
	if err != nil {
		t.Fatal(err)
	}
	if v != "literal" {
		t.Errorf("got %v", v)
	}
	v, err = ctx.Resolve(42)
	if err != nil {
		t.Fatal(err)
	}
	if v != 42 {
		t.Errorf("got %v", v)
	}
}
