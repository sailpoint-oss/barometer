package contract

import (
	"context"
	"testing"

	"github.com/sailpoint-oss/barometer/internal/runner"
	"github.com/sailpoint-oss/barometer/internal/testserver"
)

func TestRun_NilConfig(t *testing.T) {
	result, err := Run(context.Background(), nil, nil)
	if err != ErrConfigRequired {
		t.Fatalf("Run(nil) error = %v, want %v", err, ErrConfigRequired)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestRun_OpenAPIConfigSucceeds(t *testing.T) {
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)

	result, err := Run(context.Background(), &Config{
		BaseURL: baseURL,
		OpenAPI: &OpenAPIConfig{
			Spec: specURL,
		},
	}, runner.NewClient(nil))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil || result.OpenAPI == nil {
		t.Fatalf("expected OpenAPI results, got %+v", result)
	}
	if !result.Pass {
		t.Fatalf("expected passing result, got %+v", result)
	}
	if result.OpenAPI.Passed != result.OpenAPI.Total || result.OpenAPI.Total == 0 {
		t.Fatalf("unexpected OpenAPI counts: %+v", result.OpenAPI)
	}
}
