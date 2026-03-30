package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

const (
	runtimeGuidelinesBaseURL      = "https://sailpoint-oss.github.io/sailpoint-api-guidelines/"
	runtimeGuidelineStatusCodes   = "sp-403"
	runtimeGuidelineProblemDetail = "sp-404"
	runtimeGuidelineRetryability  = "sp-418"
	runtimeGuidelineRequestID     = "sp-903"
)

// GuidelineValidationError captures a runtime validation failure that maps to a
// specific SailPoint guideline.
type GuidelineValidationError struct {
	GuidelineID string
	DocURL      string
	Message     string
}

func (e *GuidelineValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// ValidateResponse checks that the HTTP response is consistent with the operation's responses.
// Status must be declared in the operation's responses; if the response has a JSON body and
// the spec defines a schema for that status, the body is validated against the schema.
func ValidateResponse(ctx context.Context, idx *navigator.Index, resp *http.Response, op *navigator.Operation, statusCode int) error {
	if op == nil || op.Responses == nil {
		return fmt.Errorf("openapi: operation and responses required")
	}
	statusStr := strconv.Itoa(statusCode)
	res, ok := op.Responses[statusStr]
	if !ok {
		res = op.Responses["default"]
	}
	if res == nil {
		return newGuidelineValidationError(runtimeGuidelineStatusCodes, "openapi: status %d not declared in responses", statusCode)
	}
	res = resolveResponse(idx, res)
	if res == nil {
		return nil
	}

	if err := validateRuntimeHeaders(resp, statusCode); err != nil {
		return err
	}

	ct := resp.Header.Get("Content-Type")
	body, data, err := readResponseBody(resp, ct)
	if err != nil {
		if isJSONContentType(ct) || isProblemJSONContentType(ct) {
			if statusCode >= http.StatusBadRequest {
				return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: response body is not valid JSON: %v", err)
			}
			return fmt.Errorf("openapi: response body is not valid JSON: %w", err)
		}
		return fmt.Errorf("openapi: read response body: %w", err)
	}

	if err := validateProblemDetails(statusCode, ct, body, data); err != nil {
		return err
	}

	if ct == "" || statusCode == 204 || len(body) == 0 {
		return nil
	}
	if !isJSONContentType(ct) && !isProblemJSONContentType(ct) {
		return nil
	}
	if res.Content == nil {
		return nil
	}
	mt := res.Content["application/json"]
	if mt == nil {
		mt = res.Content["application/problem+json"]
	}
	if mt == nil {
		mt = res.Content["*/*"]
	}
	if mt == nil || mt.Schema == nil {
		return nil
	}
	schema := resolveSchemaRef(idx, mt.Schema)
	if schema == nil {
		return nil
	}
	errs := ValidateJSON(idx, schema, data)
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		if statusCode >= http.StatusBadRequest {
			return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: response body does not match schema: %s", strings.Join(msgs, "; "))
		}
		return fmt.Errorf("openapi: response body does not match schema: %s", strings.Join(msgs, "; "))
	}
	return nil
}

func resolveResponse(idx *navigator.Index, r *navigator.Response) *navigator.Response {
	if r == nil || r.Ref == "" {
		return r
	}
	resolved, err := idx.ResolveRef(r.Ref)
	if err != nil {
		return r
	}
	if res, ok := resolved.(*navigator.Response); ok {
		return res
	}
	return r
}

func resolveSchemaRef(idx *navigator.Index, s *navigator.Schema) *navigator.Schema {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		resolved, err := idx.ResolveRef(s.Ref)
		if err != nil {
			return s
		}
		if r, ok := resolved.(*navigator.Schema); ok {
			return r
		}
	}
	return s
}

func isJSONContentType(ct string) bool {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	return strings.EqualFold(ct, "application/json")
}

func isProblemJSONContentType(ct string) bool {
	ct = strings.TrimSpace(strings.Split(ct, ";")[0])
	return strings.EqualFold(ct, "application/problem+json")
}

func readResponseBody(resp *http.Response, ct string) ([]byte, any, error) {
	if resp == nil || resp.Body == nil || resp.StatusCode == http.StatusNoContent {
		return nil, nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	if len(body) == 0 {
		return nil, nil, nil
	}
	if !isJSONContentType(ct) && !isProblemJSONContentType(ct) {
		return body, nil, nil
	}

	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return body, nil, err
	}
	return body, data, nil
}

func validateRuntimeHeaders(resp *http.Response, statusCode int) error {
	if resp == nil {
		return fmt.Errorf("openapi: response required")
	}
	if strings.TrimSpace(resp.Header.Get("X-Request-Id")) == "" {
		return newGuidelineValidationError(runtimeGuidelineRequestID, "openapi: missing X-Request-Id header")
	}

	retryAfter := strings.TrimSpace(resp.Header.Get("Retry-After"))
	switch statusCode {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		if retryAfter == "" {
			return newGuidelineValidationError(runtimeGuidelineRetryability, "openapi: status %d must include Retry-After header", statusCode)
		}
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity:
		if retryAfter != "" {
			return newGuidelineValidationError(runtimeGuidelineRetryability, "openapi: status %d must not include Retry-After header", statusCode)
		}
	}
	return nil
}

func validateProblemDetails(statusCode int, ct string, body []byte, data any) error {
	if statusCode < http.StatusBadRequest {
		return nil
	}
	if !isProblemJSONContentType(ct) {
		return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: error responses must use application/problem+json")
	}
	if len(body) == 0 {
		return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: error responses must include a Problem Details body")
	}

	obj, ok := data.(map[string]any)
	if !ok {
		return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: error responses must be valid Problem Details JSON objects")
	}

	// Runtime coverage is intentionally partial: verify the core Problem Details
	// envelope the client actually sees without requiring every optional
	// integration field the static spec enforces.
	required := []string{"title", "status", "detail"}
	var missing []string
	for _, key := range required {
		if _, ok := obj[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return newGuidelineValidationError(runtimeGuidelineProblemDetail, "openapi: Problem Details body missing fields: %s", strings.Join(missing, ", "))
	}

	return nil
}

func newGuidelineValidationError(code, format string, args ...any) error {
	return &GuidelineValidationError{
		GuidelineID: code,
		DocURL:      runtimeGuidelineDocURL(code),
		Message:     fmt.Sprintf(format, args...),
	}
}

func runtimeGuidelineDocURL(code string) string {
	switch code {
	case runtimeGuidelineStatusCodes:
		return runtimeGuidelinesBaseURL + "docs/rules/http-semantics#403"
	case runtimeGuidelineProblemDetail:
		return runtimeGuidelinesBaseURL + "docs/rules/http-semantics#404"
	case runtimeGuidelineRetryability:
		return runtimeGuidelinesBaseURL + "docs/rules/http-semantics#418"
	case runtimeGuidelineRequestID:
		return runtimeGuidelinesBaseURL + "docs/rules/operations-and-quality#903"
	default:
		return ""
	}
}

func guidelineMetadataFromError(err error) (string, string) {
	var guidelineErr *GuidelineValidationError
	if !errors.As(err, &guidelineErr) {
		return "", ""
	}
	return guidelineErr.GuidelineID, guidelineErr.DocURL
}
