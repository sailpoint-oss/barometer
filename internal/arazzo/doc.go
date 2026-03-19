// Package arazzo parses and executes Arazzo workflow documents (OpenAPI Initiative).
package arazzo

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func fetchURL(u string) ([]byte, error) {
	resp, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: status %d", u, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Doc is the root Arazzo Description (spec 1.0.x).
type Doc struct {
	Arazzo            string              `yaml:"arazzo" json:"arazzo"`
	Info              Info                `yaml:"info" json:"info"`
	SourceDescriptions []SourceDescription `yaml:"sourceDescriptions" json:"sourceDescriptions"`
	Workflows         []Workflow          `yaml:"workflows" json:"workflows"`
	Components        *Components         `yaml:"components,omitempty" json:"components,omitempty"`
}

// Info is metadata about the Arazzo document.
type Info struct {
	Title       string `yaml:"title" json:"title"`
	Summary     string `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string `yaml:"version" json:"version"`
}

// SourceDescription references an OpenAPI or Arazzo document.
type SourceDescription struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url" json:"url"`
	Type string `yaml:"type,omitempty" json:"type,omitempty"` // "openapi" or "arazzo"
}

// Workflow defines a sequence of steps.
type Workflow struct {
	WorkflowID     string    `yaml:"workflowId" json:"workflowId"`
	Summary       string    `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description   string    `yaml:"description,omitempty" json:"description,omitempty"`
	Inputs        any       `yaml:"inputs,omitempty" json:"inputs,omitempty"` // JSON Schema
	DependsOn     []string  `yaml:"dependsOn,omitempty" json:"dependsOn,omitempty"`
	Steps         []Step    `yaml:"steps" json:"steps"`
	Outputs       map[string]any `yaml:"outputs,omitempty" json:"outputs,omitempty"` // map[name]expression
	Parameters    []Parameter   `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	SuccessActions []any `yaml:"successActions,omitempty" json:"successActions,omitempty"`
	FailureActions []any `yaml:"failureActions,omitempty" json:"failureActions,omitempty"`
}

// Step is a single workflow step (operation or sub-workflow).
type Step struct {
	StepID         string            `yaml:"stepId" json:"stepId"`
	Description    string            `yaml:"description,omitempty" json:"description,omitempty"`
	OperationID    string            `yaml:"operationId,omitempty" json:"operationId,omitempty"`
	OperationPath  string            `yaml:"operationPath,omitempty" json:"operationPath,omitempty"`
	WorkflowID     string            `yaml:"workflowId,omitempty" json:"workflowId,omitempty"`
	Parameters     []Parameter       `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	RequestBody    any               `yaml:"requestBody,omitempty" json:"requestBody,omitempty"`
	SuccessCriteria []Criterion      `yaml:"successCriteria,omitempty" json:"successCriteria,omitempty"`
	OnSuccess      []any             `yaml:"onSuccess,omitempty" json:"onSuccess,omitempty"`
	OnFailure      []any             `yaml:"onFailure,omitempty" json:"onFailure,omitempty"`
	Outputs        map[string]any    `yaml:"outputs,omitempty" json:"outputs,omitempty"` // map[name]expression
}

// Parameter is a step parameter (query, header, path, cookie).
type Parameter struct {
	Name  string `yaml:"name" json:"name"`
	In    string `yaml:"in,omitempty" json:"in,omitempty"`
	Value any    `yaml:"value" json:"value"` // literal or expression string
}

// Criterion is a success/failure assertion (simple, regex, jsonpath).
type Criterion struct {
	Context   string `yaml:"context,omitempty" json:"context,omitempty"`     // runtime expression for context
	Condition string `yaml:"condition" json:"condition"`
	Type      string `yaml:"type,omitempty" json:"type,omitempty"` // "simple", "regex", "jsonpath", "xpath"
}

// Components holds reusable objects.
type Components struct {
	Inputs         map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Parameters     map[string]any `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	SuccessActions map[string]any `yaml:"successActions,omitempty" json:"successActions,omitempty"`
	FailureActions map[string]any `yaml:"failureActions,omitempty" json:"failureActions,omitempty"`
}

// Load reads an Arazzo document from path (file or URL). URLs are fetched via HTTP.
func Load(path string) (*Doc, error) {
	data, err := readPath(path)
	if err != nil {
		return nil, err
	}
	var doc Doc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("arazzo: parse: %w", err)
	}
	return &doc, nil
}

func readPath(path string) ([]byte, error) {
	if isURL(path) {
		return fetchURL(path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

// Validate checks required fields and structure.
func (d *Doc) Validate() error {
	if d.Arazzo == "" {
		return fmt.Errorf("arazzo: missing required field 'arazzo'")
	}
	if d.Info.Title == "" {
		return fmt.Errorf("arazzo: info.title is required")
	}
	if d.Info.Version == "" {
		return fmt.Errorf("arazzo: info.version is required")
	}
	if len(d.SourceDescriptions) == 0 {
		return fmt.Errorf("arazzo: sourceDescriptions must have at least one entry")
	}
	if len(d.Workflows) == 0 {
		return fmt.Errorf("arazzo: workflows must have at least one entry")
	}
	seen := make(map[string]bool)
	for i, w := range d.Workflows {
		if w.WorkflowID == "" {
			return fmt.Errorf("arazzo: workflows[%d].workflowId is required", i)
		}
		if seen[w.WorkflowID] {
			return fmt.Errorf("arazzo: duplicate workflowId %q", w.WorkflowID)
		}
		seen[w.WorkflowID] = true
		if len(w.Steps) == 0 {
			return fmt.Errorf("arazzo: workflow %q has no steps", w.WorkflowID)
		}
		stepIds := make(map[string]bool)
		for j, s := range w.Steps {
			if s.StepID == "" {
				return fmt.Errorf("arazzo: workflow %q steps[%d].stepId is required", w.WorkflowID, j)
			}
			if stepIds[s.StepID] {
				return fmt.Errorf("arazzo: duplicate stepId %q in workflow %q", s.StepID, w.WorkflowID)
			}
			stepIds[s.StepID] = true
			refs := 0
			if s.OperationID != "" {
				refs++
			}
			if s.OperationPath != "" {
				refs++
			}
			if s.WorkflowID != "" {
				refs++
			}
			if refs != 1 {
				return fmt.Errorf("arazzo: step %q must have exactly one of operationId, operationPath, workflowId", s.StepID)
			}
		}
	}
	return nil
}
