package openapi

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibOpenAPIResponseValidator_ValidateResponse(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /widgets:
    get:
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                type: object
                required:
                  - name
                properties:
                  name:
                    type: string
                    minLength: 3
`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("os.WriteFile error = %v", err)
	}

	validator, err := NewLibOpenAPIResponseValidator(context.Background(), specPath, nil)
	if err != nil {
		t.Fatalf("NewLibOpenAPIResponseValidator error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/widgets", nil)
	if err != nil {
		t.Fatalf("http.NewRequest error = %v", err)
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"name":""}`)),
	}

	errs := validator.ValidateResponse(req, resp)
	if len(errs) == 0 {
		t.Fatal("expected validation errors")
	}
	if !strings.Contains(errs[0].Error(), "name") {
		t.Fatalf("expected field information in validation error, got %q", errs[0].Error())
	}
}

func TestLibOpenAPIResponseValidator_IgnoresCircularReferenceRenderFailures(t *testing.T) {
	specPath := filepath.Join(t.TempDir(), "openapi.yaml")
	spec := `openapi: 3.1.0
info:
  title: Test API
  version: 1.0.0
paths:
  /tree:
    get:
      responses:
        '200':
          description: ok
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Node'
components:
  schemas:
    Node:
      type: object
      required:
        - value
      properties:
        value:
          type: string
        children:
          type: array
          items:
            $ref: '#/components/schemas/Node'
`
	if err := os.WriteFile(specPath, []byte(spec), 0o600); err != nil {
		t.Fatalf("os.WriteFile error = %v", err)
	}

	validator, err := NewLibOpenAPIResponseValidator(context.Background(), specPath, nil)
	if err != nil {
		t.Fatalf("NewLibOpenAPIResponseValidator error = %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "https://example.com/tree", nil)
	if err != nil {
		t.Fatalf("http.NewRequest error = %v", err)
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(`{"value":"root","children":[{"value":"leaf"}]}`)),
	}

	if errs := validator.ValidateResponse(req, resp); len(errs) != 0 {
		t.Fatalf("expected circular reference render errors to be ignored, got %v", errs)
	}
}
