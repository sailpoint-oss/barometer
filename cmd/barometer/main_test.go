package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sailpoint-oss/barometer/internal/reporter"
	"github.com/sailpoint-oss/barometer/internal/testserver"
	"github.com/spf13/cobra"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}

func TestRunSchema_PrintsEmbeddedReportSchema(t *testing.T) {
	output := captureStdout(t, func() {
		if err := runSchema(nil, nil); err != nil {
			t.Fatalf("runSchema: %v", err)
		}
	})
	if !strings.Contains(output, `"title"`) {
		t.Fatalf("expected schema JSON output, got:\n%s", output)
	}
}

func TestRunContractTest_JSONOutput(t *testing.T) {
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)

	configPath := filepath.Join(t.TempDir(), "barometer.yaml")
	config := "baseUrl: " + baseURL + "\nopenapi:\n  spec: " + specURL + "\n"
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	prevConfig := contractTestConfig
	prevOutput := contractTestOutput
	contractTestConfig = configPath
	contractTestOutput = "json"
	t.Cleanup(func() {
		contractTestConfig = prevConfig
		contractTestOutput = prevOutput
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	output := captureStdout(t, func() {
		if err := runContractTest(cmd, nil); err != nil {
			t.Fatalf("runContractTest: %v", err)
		}
	})

	var report reporter.JSONReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("unmarshal JSON output: %v\noutput:\n%s", err, output)
	}
	if report.OpenAPI == nil || report.OpenAPI.Total == 0 {
		t.Fatalf("expected OpenAPI results in JSON output, got %+v", report)
	}
	if !report.Pass {
		t.Fatalf("expected passing JSON output, got %+v", report)
	}
}
