package openapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/sailpoint-oss/barometer/internal/runner"
	"github.com/sailpoint-oss/barometer/internal/testserver"
	navigator "github.com/sailpoint-oss/navigator"
)

// startAndLoad starts the Huma test server, loads the OpenAPI spec from it, and returns baseURL, index, and HTTP client.
func startAndLoad(t *testing.T) (baseURL string, idx *navigator.Index, client *runner.Client) {
	t.Helper()
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	var err error
	idx, err = Load(ctx, specURL, nil)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := Validate(idx); err != nil {
		t.Fatalf("validate: %v", err)
	}
	return baseURL, idx, runner.NewClient(nil)
}

// runOperation runs a single operation by path and method and asserts it passes.
func runOperation(t *testing.T, ctx context.Context, idx *navigator.Index, baseURL string, client *runner.Client, path, method string) {
	t.Helper()
	opr, err := ResolveOperationByPath(idx, path, method)
	if err != nil {
		t.Fatalf("resolve operation: %v", err)
	}
	pathItem := pathItemFor(idx, path)
	pathParams := placeholderPathParams(idx, path, pathItem, opr.Operation)
	req, err := BuildRequest(ctx, idx, baseURL, path, strings.ToUpper(method), pathItem, opr.Operation, pathParams, nil, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := client.Do(ctx, req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	err = ValidateResponse(ctx, idx, resp, opr.Operation, resp.StatusCode)
	if err != nil {
		t.Fatalf("validate response: %v", err)
	}
}

func TestE2E_ListWidgets(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/widgets", http.MethodGet)
}

func TestE2E_CreateWidget(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/widgets", http.MethodPost)
}

func TestE2E_GetWidget(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/widgets/{widgetId}", http.MethodGet)
}

func TestE2E_DeleteWidget(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/widgets/{widgetId}", http.MethodDelete)
}

func TestE2E_CreateUser(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/users", http.MethodPost)
}

func TestE2E_GetUser(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/users/{userId}", http.MethodGet)
}

func TestE2E_ArrayConstraints(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/tags", http.MethodGet)
}

func TestE2E_AdditionalPropertiesFalse(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/config", http.MethodGet)
}

func TestE2E_AdditionalPropertiesTyped(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/metadata", http.MethodGet)
}

func TestE2E_QueryParams(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/search", http.MethodGet)
}

func TestE2E_HeaderParams(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/echo", http.MethodGet)
}

func TestE2E_Deprecated(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/legacy", http.MethodGet)
}

func TestE2E_Nullable(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/nullable", http.MethodGet)
}

func TestE2E_Defaults(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/defaults", http.MethodGet)
}

func TestE2E_AllOf(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/compositions/allof", http.MethodGet)
}

func TestE2E_AnyOf(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/compositions/anyof", http.MethodGet)
}

func TestE2E_OneOf(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/compositions/oneof", http.MethodGet)
}

func TestE2E_Discriminator(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/shapes", http.MethodGet)
}

func TestE2E_Recursive(t *testing.T) {
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/tree", http.MethodGet)
}

func TestE2E_StringFormats(t *testing.T) {
	// String formats (date-time, email, uuid) are exercised by CreateUser and GetUser
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/users/{userId}", http.MethodGet)
}

func TestE2E_NumberConstraints(t *testing.T) {
	// Number constraints (min/max) are exercised by CreateWidget and ListWidgets
	baseURL, idx, client := startAndLoad(t)
	ctx := context.Background()
	runOperation(t, ctx, idx, baseURL, client, "/widgets", http.MethodGet)
}

func TestE2E_FullContract(t *testing.T) {
	baseURL, specURL, cleanup := testserver.StartTestServer(t)
	defer cleanup()

	ctx := context.Background()
	idx, err := Load(ctx, specURL, nil)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if err := Validate(idx); err != nil {
		t.Fatalf("validate: %v", err)
	}
	client := runner.NewClient(nil)
	results, err := RunContract(ctx, idx, baseURL, client, nil)
	if err != nil {
		t.Fatalf("run contract: %v", err)
	}
	for _, r := range results {
		t.Run(r.Method+" "+r.Path, func(t *testing.T) {
			if !r.Pass {
				t.Errorf("FAIL: %s", r.Error)
			}
		})
	}
	if len(results) == 0 {
		t.Error("expected at least one operation")
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	if passed != len(results) {
		t.Errorf("contract: %d/%d passed", passed, len(results))
	}
}
