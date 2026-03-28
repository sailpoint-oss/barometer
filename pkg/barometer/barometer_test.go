package barometer

import (
	"context"
	"testing"
	"time"

	"github.com/sailpoint-oss/barometer/internal/contract"
	internalopenapi "github.com/sailpoint-oss/barometer/internal/openapi"
	"github.com/sailpoint-oss/barometer/internal/runner"
	"github.com/sailpoint-oss/barometer/internal/testserver"
	navigator "github.com/sailpoint-oss/navigator"
)

func loadTestIndex(t *testing.T) (string, *navigator.Index) {
	t.Helper()
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)
	idx, err := internalopenapi.Load(context.Background(), specURL, nil)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := internalopenapi.Validate(idx); err != nil {
		t.Fatalf("validate spec: %v", err)
	}
	return baseURL, idx
}

func TestStart_NilConfig(t *testing.T) {
	job, err := Start(context.Background(), nil, nil)
	if err != contract.ErrConfigRequired {
		t.Fatalf("Start(nil) error = %v, want %v", err, contract.ErrConfigRequired)
	}
	if job != nil {
		t.Fatal("expected nil job for nil config")
	}
}

func TestRunWithIndex_AndStartWithIndex(t *testing.T) {
	baseURL, idx := loadTestIndex(t)

	result, err := RunWithIndex(context.Background(), idx, baseURL, &RunOpts{
		Client:      runner.NewClient(nil),
		OperationID: "createWidget",
	})
	if err != nil {
		t.Fatalf("RunWithIndex: %v", err)
	}
	if result == nil || result.OpenAPI == nil {
		t.Fatalf("expected OpenAPI result, got %+v", result)
	}
	if !result.Pass || result.OpenAPI.Passed != result.OpenAPI.Total || result.OpenAPI.Total != 1 {
		t.Fatalf("unexpected result: %+v", result.OpenAPI)
	}

	job := StartWithIndex(context.Background(), idx, baseURL, &RunOpts{
		Client:      runner.NewClient(nil),
		OperationID: "createWidget",
	})
	if job == nil {
		t.Fatal("expected job")
	}
	asyncResult, err := job.Wait(10 * time.Second)
	if err != nil {
		t.Fatalf("job.Wait: %v", err)
	}
	state, _, statusErr := job.Status()
	if statusErr != nil {
		t.Fatalf("job.Status error = %v", statusErr)
	}
	if state != "completed" {
		t.Fatalf("job state = %q, want completed", state)
	}
	if asyncResult == nil || asyncResult.OpenAPI == nil || !asyncResult.Pass {
		t.Fatalf("unexpected async result: %+v", asyncResult)
	}
}
