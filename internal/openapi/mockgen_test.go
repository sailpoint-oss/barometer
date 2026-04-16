package openapi

import (
	"encoding/json"
	"testing"

	navigator "github.com/sailpoint-oss/navigator"
)

func TestBuildRequestBody_GeneratesSchemaValidJSON(t *testing.T) {
	idx := &navigator.Index{}
	minItems := 2
	minimum := 3.0
	schema := &navigator.Schema{
		Type:     "object",
		Required: []string{"email", "tags", "count", "nested"},
		Properties: map[string]*navigator.Schema{
			"email": {Type: "string", Format: "email"},
			"tags": {
				Type:     "array",
				MinItems: &minItems,
				Items:    &navigator.Schema{Type: "string"},
			},
			"count": {Type: "integer", Minimum: &minimum},
			"nested": {
				Type:     "object",
				Required: []string{"enabled"},
				Properties: map[string]*navigator.Schema{
					"enabled": {Type: "boolean"},
				},
			},
		},
	}

	body, contentType, err := buildRequestBody(idx, &navigator.RequestBody{
		Content: map[string]*navigator.MediaType{
			"application/json": {Schema: schema},
		},
	})
	if err != nil {
		t.Fatalf("buildRequestBody error = %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %q, want application/json", contentType)
	}

	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("json.Unmarshal error = %v\nbody=%s", err, body)
	}

	if errs := ValidateJSON(idx, schema, value); len(errs) > 0 {
		t.Fatalf("generated body did not satisfy schema: %v", errs)
	}
}

func TestBuildRequestBody_UsesJSONMediaTypeExample(t *testing.T) {
	body, contentType, err := buildRequestBody(&navigator.Index{}, &navigator.RequestBody{
		Content: map[string]*navigator.MediaType{
			"application/json": {
				Example: &navigator.Node{Value: "name: example\ncount: 2\n"},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildRequestBody error = %v", err)
	}
	if contentType != "application/json" {
		t.Fatalf("contentType = %q, want application/json", contentType)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error = %v\nbody=%s", err, body)
	}
	if decoded["name"] != "example" {
		t.Fatalf("decoded[name] = %#v, want example", decoded["name"])
	}
	if decoded["count"] != float64(2) {
		t.Fatalf("decoded[count] = %#v, want 2", decoded["count"])
	}
}
