package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

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
		return fmt.Errorf("openapi: status %d not declared in responses", statusCode)
	}
	res = resolveResponse(idx, res)
	if res == nil {
		return nil
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" || statusCode == 204 || resp.ContentLength == 0 {
		return nil
	}
	if !isJSONContentType(ct) {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("openapi: read response body: %w", err)
	}
	if len(body) == 0 {
		return nil
	}
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("openapi: response body is not valid JSON: %w", err)
	}
	if res.Content == nil {
		return nil
	}
	mt := res.Content["application/json"]
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
