package reporter

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sailpoint-oss/barometer/internal/contract"
	"github.com/sailpoint-oss/barometer/internal/openapi"
)

func TestWriteOpenAPIResults_JUnitEscapesFailures(t *testing.T) {
	results := []openapi.ContractResult{
		{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true},
		{Path: "/widgets/{id}", Method: "GET", OperationID: "getWidget", Pass: false, Error: `expected <ok> & "quoted"`},
	}

	var out bytes.Buffer
	if err := WriteOpenAPIResults(&out, results, FormatJUnit, 1500*time.Millisecond); err != nil {
		t.Fatalf("WriteOpenAPIResults: %v", err)
	}
	report := out.String()

	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`tests="2"`,
		`failures="1"`,
		`expected &lt;ok&gt; &amp; &quot;quoted&quot;`,
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("JUnit output missing %q:\n%s", want, report)
		}
	}
}

func TestWriteOpenAPIResults_JSONShape(t *testing.T) {
	results := []openapi.ContractResult{
		{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true, Status: 200},
		{Path: "/widgets", Method: "POST", OperationID: "createWidget", Pass: false, Status: 400, Error: "bad request"},
	}

	var out bytes.Buffer
	if err := WriteOpenAPIResults(&out, results, FormatJSON, 250*time.Millisecond); err != nil {
		t.Fatalf("WriteOpenAPIResults: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Version != JSONReportVersion {
		t.Fatalf("version = %q, want %q", report.Version, JSONReportVersion)
	}
	if report.Pass {
		t.Fatalf("expected failing JSON report, got %+v", report)
	}
	if report.OpenAPI == nil || report.OpenAPI.Passed != 1 || report.OpenAPI.Total != 2 {
		t.Fatalf("unexpected OpenAPI section: %+v", report.OpenAPI)
	}
}

func TestWriteContractResult_IncludesOpenAPIAndArazzo(t *testing.T) {
	result := &contract.Result{
		Pass: false,
		OpenAPI: &contract.OpenAPIResult{
			Passed: 1,
			Total:  2,
			Results: []openapi.ContractResult{
				{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true, Status: 200},
				{Path: "/widgets", Method: "POST", OperationID: "createWidget", Pass: false, Status: 500, Error: "boom"},
			},
		},
		Arazzo: &contract.ArazzoResult{
			Passed: 0,
			Total:  1,
			Workflows: []contract.WorkflowResult{
				{WorkflowID: "syncWidgets", Pass: false, Error: "step failed"},
			},
		},
	}

	var out bytes.Buffer
	if err := WriteContractResult(&out, result, 2*time.Second); err != nil {
		t.Fatalf("WriteContractResult: %v", err)
	}

	var report JSONReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Pass {
		t.Fatalf("expected failing report, got %+v", report)
	}
	if report.OpenAPI == nil || report.OpenAPI.Total != 2 {
		t.Fatalf("unexpected OpenAPI report: %+v", report.OpenAPI)
	}
	if report.Arazzo == nil || report.Arazzo.Total != 1 || len(report.Arazzo.Workflows) != 1 {
		t.Fatalf("unexpected Arazzo report: %+v", report.Arazzo)
	}
}

func TestWriteContractReport_JUnitIncludesOpenAPIAndArazzo(t *testing.T) {
	result := &contract.Result{
		Pass: false,
		OpenAPI: &contract.OpenAPIResult{
			Passed: 1,
			Total:  2,
			Results: []openapi.ContractResult{
				{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true},
				{Path: "/widgets/{id}", Method: "GET", OperationID: "getWidget", Pass: false, Error: `expected <ok> & "quoted"`},
			},
		},
		Arazzo: &contract.ArazzoResult{
			Passed: 0,
			Total:  1,
			Workflows: []contract.WorkflowResult{
				{WorkflowID: "syncWidgets", Pass: false, Error: "step <failed>"},
			},
		},
	}

	var out bytes.Buffer
	if err := WriteContractReport(&out, result, FormatJUnit, 1500*time.Millisecond); err != nil {
		t.Fatalf("WriteContractReport: %v", err)
	}
	report := out.String()

	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<testsuite name="openapi" tests="2" failures="1"`,
		`<testsuite name="arazzo" tests="1" failures="1"`,
		`expected &lt;ok&gt; &amp; &quot;quoted&quot;`,
		`step &lt;failed&gt;`,
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("combined JUnit output missing %q:\n%s", want, report)
		}
	}
}

func TestBuildContractReport(t *testing.T) {
	result := &contract.Result{
		Pass: true,
		OpenAPI: &contract.OpenAPIResult{
			Passed: 1,
			Total:  1,
			Results: []openapi.ContractResult{
				{Path: "/widgets", Method: "GET", OperationID: "listWidgets", Pass: true, Status: 200},
			},
		},
	}

	report := BuildContractReport(result, 375*time.Millisecond)
	if report.Version != JSONReportVersion {
		t.Fatalf("version = %q, want %q", report.Version, JSONReportVersion)
	}
	if !report.Pass || report.DurationMs != 375 {
		t.Fatalf("unexpected report envelope: %+v", report)
	}
	if report.OpenAPI == nil || report.OpenAPI.Total != 1 || report.Arazzo != nil {
		t.Fatalf("unexpected report sections: %+v", report)
	}
}
