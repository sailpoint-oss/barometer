package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/sailpoint-oss/barometer/internal/contract"
	"github.com/sailpoint-oss/barometer/internal/openapi"
)

// Format is the output format.
type Format string

const (
	FormatHuman Format = "human"
	FormatJUnit Format = "junit"
	FormatJSON  Format = "json"
)

// WriteOpenAPIResults writes OpenAPI contract results in the given format.
func WriteOpenAPIResults(w io.Writer, results []openapi.ContractResult, format Format, duration time.Duration) error {
	switch format {
	case FormatHuman:
		return writeOpenAPIHuman(w, results, duration)
	case FormatJUnit:
		return writeOpenAPIJUnit(w, results, duration)
	case FormatJSON:
		return writeOpenAPIJSON(w, results, duration)
	default:
		return writeOpenAPIHuman(w, results, duration)
	}
}

func writeOpenAPIHuman(w io.Writer, results []openapi.ContractResult, duration time.Duration) error {
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		} else {
			fmt.Fprintf(w, "FAIL %s %s: %s\n", r.Method, r.Path, r.Error)
		}
	}
	fmt.Fprintf(w, "\nOpenAPI contract: %d/%d passed in %v\n", passed, len(results), duration.Round(time.Millisecond))
	return nil
}

func writeOpenAPIJUnit(w io.Writer, results []openapi.ContractResult, duration time.Duration) error {
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	// Minimal JUnit XML
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="barometer" tests="%d" failures="%d" time="%.3f">
  <testsuite name="openapi" tests="%d" failures="%d" time="%.3f">
`, len(results), len(results)-passed, duration.Seconds(), len(results), len(results)-passed, duration.Seconds())
	for _, r := range results {
		name := r.OperationID
		if name == "" {
			name = r.Method + " " + r.Path
		}
		class := strings.ReplaceAll(r.Path, "/", ".")
		if !r.Pass {
			fmt.Fprintf(w, `    <testcase classname="%s" name="%s" time="0">
      <failure message="%s">%s</failure>
    </testcase>
`, escapeXML(class), escapeXML(name), escapeXML(r.Error), escapeXML(r.Error))
		} else {
			fmt.Fprintf(w, `    <testcase classname="%s" name="%s" time="0"/>
`, escapeXML(class), escapeXML(name))
		}
	}
	fmt.Fprint(w, "  </testsuite>\n</testsuites>\n")
	return nil
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// JSONReport is the versioned JSON output schema for programmatic consumers
// such as CI, GitHub Actions, and editor tooling.
const JSONReportVersion = "1.0"

type JSONReport struct {
	Version    string         `json:"version"`
	Pass       bool           `json:"pass"`
	OpenAPI    *OpenAPIReport `json:"openapi,omitempty"`
	Arazzo     *ArazzoReport  `json:"arazzo,omitempty"`
	DurationMs int64          `json:"durationMs,omitempty"`
}

type OpenAPIReport struct {
	Passed  int                      `json:"passed"`
	Total   int                      `json:"total"`
	Results []openapi.ContractResult `json:"results"`
}

type ArazzoReport struct {
	Passed    int                       `json:"passed"`
	Total     int                       `json:"total"`
	Workflows []contract.WorkflowResult `json:"workflows"`
}

func writeOpenAPIJSON(w io.Writer, results []openapi.ContractResult, duration time.Duration) error {
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(JSONReport{
		Version:    JSONReportVersion,
		Pass:       passed == len(results),
		DurationMs: duration.Milliseconds(),
		OpenAPI: &OpenAPIReport{
			Passed:  passed,
			Total:   len(results),
			Results: results,
		},
	})
}

// WriteContractResult writes full contract result (OpenAPI + Arazzo) as JSON.
func WriteContractResult(w io.Writer, result *contract.Result, duration time.Duration) error {
	report := JSONReport{
		Version:    JSONReportVersion,
		Pass:       result.Pass,
		DurationMs: duration.Milliseconds(),
	}
	if result.OpenAPI != nil {
		report.OpenAPI = &OpenAPIReport{
			Passed:  result.OpenAPI.Passed,
			Total:   result.OpenAPI.Total,
			Results: result.OpenAPI.Results,
		}
	}
	if result.Arazzo != nil {
		report.Arazzo = &ArazzoReport{
			Passed:    result.Arazzo.Passed,
			Total:     result.Arazzo.Total,
			Workflows: result.Arazzo.Workflows,
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
