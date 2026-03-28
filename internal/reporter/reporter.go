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

// WriteContractReport writes combined OpenAPI and Arazzo contract results in the
// requested format.
func WriteContractReport(w io.Writer, result *contract.Result, format Format, duration time.Duration) error {
	switch format {
	case FormatHuman:
		return writeContractHuman(w, result, duration)
	case FormatJUnit:
		return writeContractJUnit(w, result, duration)
	case FormatJSON:
		return WriteContractResult(w, result, duration)
	default:
		return writeContractHuman(w, result, duration)
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

func writeContractHuman(w io.Writer, result *contract.Result, duration time.Duration) error {
	if result == nil {
		result = &contract.Result{}
	}
	wroteSection := false
	if result.OpenAPI != nil {
		for _, r := range result.OpenAPI.Results {
			if !r.Pass {
				fmt.Fprintf(w, "FAIL %s %s: %s\n", r.Method, r.Path, r.Error)
			}
		}
		fmt.Fprintf(w, "\nOpenAPI contract: %d/%d passed\n", result.OpenAPI.Passed, result.OpenAPI.Total)
		wroteSection = true
	}
	if result.Arazzo != nil {
		if wroteSection {
			fmt.Fprintln(w)
		}
		for _, wf := range result.Arazzo.Workflows {
			if !wf.Pass {
				fmt.Fprintf(w, "FAIL workflow %s: %s\n", wf.WorkflowID, wf.Error)
			}
		}
		fmt.Fprintf(w, "\nArazzo workflows: %d/%d passed\n", result.Arazzo.Passed, result.Arazzo.Total)
		wroteSection = true
	}
	status := "passed"
	if !result.Pass {
		status = "failed"
	}
	if wroteSection {
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Combined contract: %s in %v\n", status, duration.Round(time.Millisecond))
	return nil
}

func writeContractJUnit(w io.Writer, result *contract.Result, duration time.Duration) error {
	if result == nil {
		result = &contract.Result{}
	}
	total := 0
	failures := 0
	if result.OpenAPI != nil {
		total += result.OpenAPI.Total
		failures += result.OpenAPI.Total - result.OpenAPI.Passed
	}
	if result.Arazzo != nil {
		total += result.Arazzo.Total
		failures += result.Arazzo.Total - result.Arazzo.Passed
	}
	fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<testsuites name="barometer" tests="%d" failures="%d" time="%.3f">
`, total, failures, duration.Seconds())
	if result.OpenAPI != nil {
		fmt.Fprintf(w, `  <testsuite name="openapi" tests="%d" failures="%d" time="0">
`, result.OpenAPI.Total, result.OpenAPI.Total-result.OpenAPI.Passed)
		for _, r := range result.OpenAPI.Results {
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
		fmt.Fprint(w, "  </testsuite>\n")
	}
	if result.Arazzo != nil {
		fmt.Fprintf(w, `  <testsuite name="arazzo" tests="%d" failures="%d" time="0">
`, result.Arazzo.Total, result.Arazzo.Total-result.Arazzo.Passed)
		for _, wf := range result.Arazzo.Workflows {
			if !wf.Pass {
				fmt.Fprintf(w, `    <testcase classname="arazzo" name="%s" time="0">
      <failure message="%s">%s</failure>
    </testcase>
`, escapeXML(wf.WorkflowID), escapeXML(wf.Error), escapeXML(wf.Error))
			} else {
				fmt.Fprintf(w, `    <testcase classname="arazzo" name="%s" time="0"/>
`, escapeXML(wf.WorkflowID))
			}
		}
		fmt.Fprint(w, "  </testsuite>\n")
	}
	fmt.Fprint(w, "</testsuites>\n")
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

// BuildContractReport builds the versioned JSON report structure for programmatic consumers.
func BuildContractReport(result *contract.Result, duration time.Duration) JSONReport {
	if result == nil {
		result = &contract.Result{}
	}
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
	return report
}

// WriteContractResult writes full contract result (OpenAPI + Arazzo) as JSON.
func WriteContractResult(w io.Writer, result *contract.Result, duration time.Duration) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(BuildContractReport(result, duration))
}
