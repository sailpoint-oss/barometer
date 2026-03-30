package openapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	navigator "github.com/sailpoint-oss/navigator"
)

const requestID = "123e4567-e89b-12d3-a456-426614174000"

func TestValidateResponse_MissingDeclaredStatusUsesSP403(t *testing.T) {
	idx, op := loadTestOperation(t, `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        "200":
          description: OK
`)

	err := ValidateResponse(context.Background(), idx, newTestResponse(http.StatusNotFound, "application/problem+json", `{"type":"about:blank"}`, map[string]string{
		"X-Request-Id": requestID,
	}), op, http.StatusNotFound)

	assertGuidelineError(t, err, runtimeGuidelineStatusCodes)
}

func TestValidateResponse_ErrorResponsesMustUseProblemDetails(t *testing.T) {
	idx, op := loadTestOperation(t, `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /widgets:
    get:
      operationId: getWidget
      responses:
        "404":
          description: Not Found
          content:
            application/problem+json:
              schema:
                type: object
                required:
                  - type
                  - title
                  - status
                  - detail
                  - instance
                  - correlationId
                properties:
                  type:
                    type: string
                  title:
                    type: string
                  status:
                    type: integer
                  detail:
                    type: string
                  instance:
                    type: string
                  correlationId:
                    type: string
`)

	err := ValidateResponse(context.Background(), idx, newTestResponse(http.StatusNotFound, "application/json", `{"error":"missing"}`, map[string]string{
		"X-Request-Id": requestID,
	}), op, http.StatusNotFound)

	assertGuidelineError(t, err, runtimeGuidelineProblemDetail)
}

func TestValidateResponse_RetryableErrorsRequireRetryAfter(t *testing.T) {
	idx, op := loadTestOperation(t, `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        "429":
          description: Too Many Requests
          content:
            application/problem+json:
              schema:
                type: object
                required:
                  - type
                  - title
                  - status
                  - detail
                  - instance
                  - correlationId
                properties:
                  type:
                    type: string
                  title:
                    type: string
                  status:
                    type: integer
                  detail:
                    type: string
                  instance:
                    type: string
                  correlationId:
                    type: string
`)

	err := ValidateResponse(context.Background(), idx, newTestResponse(http.StatusTooManyRequests, "application/problem+json", `{"type":"about:blank","title":"Too Many Requests","status":429,"detail":"slow down","instance":"/widgets","correlationId":"corr-1"}`, map[string]string{
		"X-Request-Id": requestID,
	}), op, http.StatusTooManyRequests)

	assertGuidelineError(t, err, runtimeGuidelineRetryability)
}

func TestValidateResponse_RequiresRequestIDHeader(t *testing.T) {
	idx, op := loadTestOperation(t, `openapi: "3.1.0"
info:
  title: Test
  version: "1.0"
paths:
  /widgets:
    get:
      operationId: listWidgets
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: object
`)

	err := ValidateResponse(context.Background(), idx, newTestResponse(http.StatusOK, "application/json", `{}`, nil), op, http.StatusOK)

	assertGuidelineError(t, err, runtimeGuidelineRequestID)
}

func loadTestOperation(t *testing.T, spec string) (*navigator.Index, *navigator.Operation) {
	t.Helper()

	idx := navigator.ParseContent([]byte(spec), "file:///spec.yaml")
	if idx == nil || idx.Document == nil {
		t.Fatal("expected parsed OpenAPI index")
	}
	opr, err := ResolveOperationByPath(idx, "/widgets", http.MethodGet)
	if err != nil {
		t.Fatalf("ResolveOperationByPath: %v", err)
	}
	if opr == nil || opr.Operation == nil {
		t.Fatal("expected resolved operation")
	}
	return idx, opr.Operation
}

func newTestResponse(status int, contentType, body string, headers map[string]string) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		header.Set(key, value)
	}
	return &http.Response{
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func assertGuidelineError(t *testing.T, err error, wantID string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected validation error")
	}
	gotID, gotURL := guidelineMetadataFromError(err)
	if gotID != wantID {
		t.Fatalf("guideline id = %q, want %q (err=%v)", gotID, wantID, err)
	}
	if gotURL == "" {
		t.Fatalf("expected doc URL for %q", wantID)
	}
}
