package contract

import (
	"context"
	"errors"
	"fmt"

	"github.com/sailpoint-oss/barometer/internal/arazzo"
	"github.com/sailpoint-oss/barometer/internal/openapi"
	"github.com/sailpoint-oss/barometer/internal/runner"
)

var ErrConfigRequired = errors.New("config is required")

// Result holds the combined result of a contract test run (OpenAPI + Arazzo).
type Result struct {
	OpenAPI *OpenAPIResult  `json:"openapi,omitempty"`
	Arazzo  *ArazzoResult   `json:"arazzo,omitempty"`
	Pass    bool            `json:"pass"`
}

// OpenAPIResult is the result of OpenAPI contract tests.
type OpenAPIResult struct {
	Results []openapi.ContractResult `json:"results"`
	Passed  int                     `json:"passed"`
	Total   int                     `json:"total"`
}

// ArazzoResult is the result of Arazzo workflow runs.
type ArazzoResult struct {
	Workflows []WorkflowResult `json:"workflows"`
	Passed    int              `json:"passed"`
	Total     int              `json:"total"`
}

// WorkflowResult is the result of one workflow run.
type WorkflowResult struct {
	WorkflowID string         `json:"workflowId"`
	Pass       bool           `json:"pass"`
	Error      string         `json:"error,omitempty"`
	Outputs    map[string]any `json:"outputs,omitempty"`
}

// Run executes contract tests and/or Arazzo workflows from config.
func Run(ctx context.Context, cfg *Config, client *runner.Client) (*Result, error) {
	if cfg == nil {
		return nil, ErrConfigRequired
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if client == nil {
		var err error
		client, err = runner.NewClient(nil)
		if err != nil {
			return nil, err
		}
	}
	out := &Result{Pass: true}
	if cfg.OpenAPI != nil {
		idx, err := openapi.Load(ctx, cfg.OpenAPI.Spec, nil)
		if err != nil {
			return nil, fmt.Errorf("openapi load: %w", err)
		}
		if err := openapi.Validate(idx); err != nil {
			return nil, fmt.Errorf("openapi validate: %w", err)
		}
		opts := &openapi.ContractOpts{Tags: cfg.OpenAPI.Tags}
		results, err := openapi.RunContract(ctx, idx, baseURL, client, opts)
		if err != nil {
			return nil, fmt.Errorf("openapi contract: %w", err)
		}
		passed := 0
		for _, r := range results {
			if r.Pass {
				passed++
			}
		}
		out.OpenAPI = &OpenAPIResult{Results: results, Passed: passed, Total: len(results)}
		if passed < len(results) {
			out.Pass = false
		}
	}
	if cfg.Arazzo != nil {
		doc, err := arazzo.Load(cfg.Arazzo.Doc)
		if err != nil {
			return nil, fmt.Errorf("arazzo load: %w", err)
		}
		if err := doc.Validate(); err != nil {
			return nil, fmt.Errorf("arazzo validate: %w", err)
		}
		workflowIDs := cfg.Arazzo.Workflows
		if len(workflowIDs) == 0 {
			for _, w := range doc.Workflows {
				workflowIDs = append(workflowIDs, w.WorkflowID)
			}
		}
		var wfResults []WorkflowResult
		passed := 0
		for _, wfID := range workflowIDs {
			outputs, err := doc.RunWorkflow(ctx, wfID, baseURL, nil, client)
			pass := err == nil
			if pass {
				passed++
			}
			errStr := ""
			if err != nil {
				errStr = err.Error()
				out.Pass = false
			}
			wfResults = append(wfResults, WorkflowResult{
				WorkflowID: wfID,
				Pass:      pass,
				Error:     errStr,
				Outputs:   outputs,
			})
		}
		out.Arazzo = &ArazzoResult{Workflows: wfResults, Passed: passed, Total: len(wfResults)}
	}
	return out, nil
}
