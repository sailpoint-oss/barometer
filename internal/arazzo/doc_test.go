package arazzo

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "arazzo", "simple.yaml")
	doc, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Arazzo == "" {
		t.Error("arazzo version empty")
	}
	if err := doc.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(doc.Workflows) != 1 || doc.Workflows[0].WorkflowID != "getPets" {
		t.Errorf("workflows: %v", doc.Workflows)
	}
}

func TestValidate_MissingWorkflowId(t *testing.T) {
	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "T", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{{Name: "api", URL: "http://example.com/spec.json", Type: "openapi"}},
		Workflows: []Workflow{
			{WorkflowID: "", Steps: []Step{{StepID: "a", OperationID: "op1", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}}}},
		},
	}
	err := doc.Validate()
	if err == nil {
		t.Fatal("expected error for missing workflowId")
	}
	if err.Error() == "" {
		t.Error("error message empty")
	}
}

func TestValidate_DuplicateStepId(t *testing.T) {
	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "T", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{{Name: "api", URL: "http://example.com/spec.json", Type: "openapi"}},
		Workflows: []Workflow{
			{
				WorkflowID: "w1",
				Steps: []Step{
					{StepID: "same", OperationID: "op1", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}},
					{StepID: "same", OperationID: "op2", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}},
				},
			},
		},
	}
	err := doc.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate stepId")
	}
}

func TestValidate_StepNeitherOperationIdNorOperationPathNorWorkflowId(t *testing.T) {
	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "T", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{{Name: "api", URL: "http://example.com/spec.json", Type: "openapi"}},
		Workflows: []Workflow{
			{
				WorkflowID: "w1",
				Steps: []Step{
					{StepID: "s1", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}},
				},
			},
		},
	}
	err := doc.Validate()
	if err == nil {
		t.Fatal("expected error when step has no operationId/operationPath/workflowId")
	}
}

func TestValidate_DuplicateWorkflowId(t *testing.T) {
	doc := &Doc{
		Arazzo: "1.0.1",
		Info:   Info{Title: "T", Version: "1.0.0"},
		SourceDescriptions: []SourceDescription{{Name: "api", URL: "http://example.com/spec.json", Type: "openapi"}},
		Workflows: []Workflow{
			{WorkflowID: "w1", Steps: []Step{{StepID: "a", OperationID: "op1", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}}}},
			{WorkflowID: "w1", Steps: []Step{{StepID: "b", OperationID: "op2", SuccessCriteria: []Criterion{{Condition: "$statusCode == 200"}}}}},
		},
	}
	err := doc.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate workflowId")
	}
}
