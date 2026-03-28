package barometer

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestContractInputToConfig(t *testing.T) {
	input := ContractInput{
		BaseURL:         "https://api.example.com",
		Output:          FormatJSON,
		OpenAPISpec:     "./openapi.yaml",
		OpenAPITags:     []string{"widgets"},
		ArazzoDoc:       "./arazzo.yaml",
		ArazzoWorkflows: []string{"syncWidgets"},
	}

	cfg, err := input.ToConfig()
	if err != nil {
		t.Fatalf("ToConfig: %v", err)
	}
	if cfg.BaseURL != input.BaseURL || cfg.Output != string(FormatJSON) {
		t.Fatalf("unexpected config envelope: %+v", cfg)
	}
	if cfg.OpenAPI == nil || cfg.OpenAPI.Spec != input.OpenAPISpec || len(cfg.OpenAPI.Tags) != 1 {
		t.Fatalf("unexpected openapi config: %+v", cfg.OpenAPI)
	}
	if cfg.Arazzo == nil || cfg.Arazzo.Doc != input.ArazzoDoc || len(cfg.Arazzo.Workflows) != 1 {
		t.Fatalf("unexpected arazzo config: %+v", cfg.Arazzo)
	}
}

func TestContractInputToConfigRequiresTarget(t *testing.T) {
	if _, err := (ContractInput{}).ToConfig(); err != ErrTargetRequired {
		t.Fatalf("ToConfig() error = %v, want %v", err, ErrTargetRequired)
	}
}

func TestWriteReportJSON(t *testing.T) {
	result := &Result{
		Pass: true,
		OpenAPI: &OpenAPIResult{
			Passed: 1,
			Total:  1,
			Results: []OpenAPIContractResult{
				{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true, Status: 200},
			},
		},
	}

	var out bytes.Buffer
	if err := WriteReport(&out, result, FormatJSON, 250*time.Millisecond); err != nil {
		t.Fatalf("WriteReport: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if !report.Pass || report.Version != JSONReportVersion || report.DurationMs != 250 {
		t.Fatalf("unexpected report envelope: %+v", report)
	}
	if report.OpenAPI == nil || report.OpenAPI.Total != 1 || report.Arazzo != nil {
		t.Fatalf("unexpected report sections: %+v", report)
	}
}
