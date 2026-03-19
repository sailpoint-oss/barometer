package openapi

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	ctx := context.Background()
	specPath := filepath.Join("..", "..", "testdata", "openapi", "petstore-minimal.yaml")
	idx, err := Load(ctx, specPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if Version(idx) != "3.0.3" {
		t.Errorf("version: got %s", Version(idx))
	}
	if err := Validate(idx); err != nil {
		t.Fatal(err)
	}
}
