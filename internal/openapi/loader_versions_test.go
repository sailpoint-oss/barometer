package openapi

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_RejectsSwagger20(t *testing.T) {
	ctx := context.Background()
	specPath := filepath.Join("..", "..", "testdata", "openapi", "swagger-2.0.yaml")

	_, err := Load(ctx, specPath, nil)
	if err == nil {
		t.Fatal("expected Swagger/OAS 2.0 fixture to be rejected")
	}
	if !strings.Contains(err.Error(), "Swagger/OAS 2.0 is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}
