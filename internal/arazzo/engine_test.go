package arazzo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/sailpoint-oss/barometer/internal/runner"
)

// stubPetServer starts a server that serves the petstore-minimal spec and GET /pets returning [].
func stubPetServer(t *testing.T) (baseURL, specURL string, cleanup func()) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "openapi", "petstore-minimal.yaml"))
	if err != nil {
		t.Fatalf("read petstore-minimal: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})
	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	srv := httptest.NewServer(mux)
	return srv.URL, srv.URL + "/openapi.json", srv.Close
}

func TestRunWorkflow_Simple(t *testing.T) {
	baseURL, specURL, cleanup := stubPetServer(t)
	t.Cleanup(cleanup)

	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "Test", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{
			{Name: "api", URL: specURL, Type: "openapi"},
		},
		Workflows: []Workflow{
			{
				WorkflowID: "getPets",
				Steps: []Step{
					{
						StepID:      "list",
						OperationID: "listPets",
						SuccessCriteria: []Criterion{
							{Condition: "$statusCode == 200"},
						},
						Outputs: map[string]any{},
					},
				},
			},
		},
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	ctx := context.Background()
	client, err := runner.NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	out, err := doc.RunWorkflow(ctx, "getPets", baseURL, nil, client)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if out == nil {
		out = make(map[string]any)
	}
	// workflow has no outputs defined; out may be empty
	_ = out
}

func TestRunWorkflow_WorkflowParamsMerge(t *testing.T) {
	var headerSeen string
	specData, err := os.ReadFile(filepath.Join("..", "..", "testdata", "openapi", "petstore-with-header.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(specData)
	})
	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		headerSeen = r.Header.Get("X-Custom")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	baseURL := srv.URL
	specURL := srv.URL + "/openapi.json"

	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "Test", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{
			{Name: "api", URL: specURL, Type: "openapi"},
		},
		Workflows: []Workflow{
			{
				WorkflowID: "getPets",
				Parameters: []Parameter{
					{Name: "X-Custom", In: "header", Value: "workflow-value"},
				},
				Steps: []Step{
					{
						StepID:      "list",
						OperationID: "listPets",
						SuccessCriteria: []Criterion{
							{Condition: "$statusCode == 200"},
						},
						Outputs: map[string]any{},
					},
				},
			},
		},
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	ctx := context.Background()
	client, err := runner.NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = doc.RunWorkflow(ctx, "getPets", baseURL, nil, client)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if headerSeen != "workflow-value" {
		t.Errorf("header X-Custom: got %q want workflow-value", headerSeen)
	}

	// Step override: same param on step should override workflow
	headerSeen = ""
	doc.Workflows[0].Steps[0].Parameters = []Parameter{
		{Name: "X-Custom", In: "header", Value: "step-value"},
	}
	_, err = doc.RunWorkflow(ctx, "getPets", baseURL, nil, client)
	if err != nil {
		t.Fatalf("RunWorkflow (step override): %v", err)
	}
	if headerSeen != "step-value" {
		t.Errorf("header X-Custom after step override: got %q want step-value", headerSeen)
	}
}

func TestRunWorkflow_StepRequestBody(t *testing.T) {
	var postBody []byte
	specWithPost := `{"openapi":"3.0.3","info":{"title":"x","version":"1.0"},"paths":{"/pets":{"get":{"operationId":"listPets","responses":{"200":{"description":"OK"}}},"post":{"operationId":"createPet","requestBody":{"content":{"application/json":{"schema":{"type":"object"}}}},"responses":{"201":{"description":"Created"}}}}}}`
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(specWithPost))
	})
	mux.HandleFunc("/pets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postBody, _ = io.ReadAll(r.Body)
			r.Body.Close()
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	baseURL := srv.URL
	specURL := srv.URL + "/openapi.json"

	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "Test", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{
			{Name: "api", URL: specURL, Type: "openapi"},
		},
		Workflows: []Workflow{
			{
				WorkflowID: "createThenList",
				Steps: []Step{
					{
						StepID:      "create",
						OperationID: "createPet",
						RequestBody: map[string]any{
							"payload": map[string]any{
								"name": "$inputs.petName",
							},
						},
						SuccessCriteria: []Criterion{
							{Condition: "$statusCode == 201"},
						},
						Outputs: map[string]any{},
					},
					{
						StepID:      "list",
						OperationID: "listPets",
						SuccessCriteria: []Criterion{
							{Condition: "$statusCode == 200"},
						},
						Outputs: map[string]any{},
					},
				},
			},
		},
	}
	if err := doc.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	ctx := context.Background()
	client, err := runner.NewClient(nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	inputs := map[string]any{"petName": "Felix"}
	_, err = doc.RunWorkflow(ctx, "createThenList", baseURL, inputs, client)
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	// Check that POST body was the resolved payload
	if len(postBody) == 0 {
		t.Error("expected POST body to be set")
	}
	if string(postBody) != `{"name":"Felix"}` {
		t.Errorf("POST body: got %s want {\"name\":\"Felix\"}", postBody)
	}
}
