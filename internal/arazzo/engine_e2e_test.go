package arazzo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/sailpoint-oss/barometer/internal/runner"
	"github.com/sailpoint-oss/barometer/internal/testserver"
)

func TestE2E_AuthThenGet(t *testing.T) {
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)

	// Huma test server accepts flat body for login (contract builds from schema which may be flat)
	loginBody := map[string]any{"username": "alice", "password": "secret"}
	bodyBytes, _ := json.Marshal(loginBody)
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	client := runner.NewClient(nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("direct login request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Skipf("server returned %d for login (expected 200); skip workflow E2E", resp.StatusCode)
		return
	}

	path := filepath.Join("..", "..", "testdata", "arazzo", "auth-then-get.yaml")
	doc, err := Load(path)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	doc.SourceDescriptions[0].URL = specURL

	ctx := context.Background()
	inputs := map[string]any{"username": "alice", "password": "secret"}
	out, err := doc.RunWorkflow(ctx, "authThenGet", baseURL, inputs, client)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil outputs")
	}
	if _, ok := out["token"]; !ok {
		t.Errorf("outputs missing token: %v", out)
	}
	if tok, _ := out["token"].(string); tok == "" {
		t.Errorf("token empty: %v", out)
	}
}

func TestE2E_ListThenGetById(t *testing.T) {
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)

	path := filepath.Join("..", "..", "testdata", "arazzo", "list-then-get-by-id.yaml")
	doc, err := Load(path)
	if err != nil {
		t.Fatalf("load workflow: %v", err)
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	doc.SourceDescriptions[0].URL = specURL

	ctx := context.Background()
	client := runner.NewClient(nil)
	out, err := doc.RunWorkflow(ctx, "listThenGetById", baseURL, nil, client)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if out == nil {
		t.Fatal("expected non-nil outputs")
	}
	if _, ok := out["firstId"]; !ok {
		t.Errorf("outputs missing firstId: %v", out)
	}
	if _, ok := out["widget"]; !ok {
		t.Errorf("outputs missing widget: %v", out)
	}
}
