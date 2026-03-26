package openapi

import (
	"context"
	"path/filepath"
	"testing"
)

func TestToolchainFixtureMatrix_LoadAndValidate(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		title       string
		wantVersion string
	}{
		{
			name:        "minimal root",
			file:        "petstore-minimal.yaml",
			title:       "Petstore minimal",
			wantVersion: "3.0.3",
		},
		{
			name:        "header parameter root",
			file:        "petstore-with-header.yaml",
			title:       "Petstore with header",
			wantVersion: "3.0.3",
		},
		{
			name:        "openapi 3.1 root",
			file:        "petstore-3.1.yaml",
			title:       "Petstore 3.1 minimal",
			wantVersion: "3.1.0",
		},
		{
			name:        "openapi 3.2 root",
			file:        "petstore-3.2.yaml",
			title:       "Petstore 3.2 minimal",
			wantVersion: "3.2.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			specPath := filepath.Join("..", "..", "testdata", "openapi", tc.file)
			idx, err := Load(context.Background(), specPath, nil)
			if err != nil {
				t.Fatalf("Load(%q): %v", specPath, err)
			}
			if Version(idx) != tc.wantVersion {
				t.Fatalf("version = %q, want %q", Version(idx), tc.wantVersion)
			}
			if err := Validate(idx); err != nil {
				t.Fatalf("Validate(%q): %v", specPath, err)
			}
			opr, err := ResolveOperationByPath(idx, "/pets", "get")
			if err != nil {
				t.Fatalf("ResolveOperationByPath(%q): %v", specPath, err)
			}
			if opr.Operation == nil || opr.Operation.OperationID != "listPets" {
				t.Fatalf("operationId = %+v, want %q", opr.Operation, "listPets")
			}
			if idx.Document == nil || idx.Document.Info == nil || idx.Document.Info.Title != tc.title {
				t.Fatalf("info.title = %+v, want %q", idx.Document, tc.title)
			}
		})
	}
}
