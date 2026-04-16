package openapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sailpoint-oss/barometer/internal/runner"
	navigator "github.com/sailpoint-oss/navigator"
)

// ContractResult is the result of a single operation contract check.
type ContractResult struct {
	Path        string `json:"path"`
	Method      string `json:"method"`
	OperationID string `json:"operationId"`
	Pass        bool   `json:"pass"`
	Status      int    `json:"status"`
	Error       string `json:"error,omitempty"`
	GuidelineID string `json:"guidelineId,omitempty"`
	DocURL      string `json:"docUrl,omitempty"`
}

// RunContract runs contract tests for all (or filtered) operations in the spec against baseURL.
func RunContract(ctx context.Context, idx *navigator.Index, baseURL string, client *runner.Client, opts *ContractOpts) ([]ContractResult, error) {
	if idx == nil || !idx.IsOpenAPI() {
		return nil, fmt.Errorf("openapi: no valid document loaded")
	}
	if opts == nil {
		opts = &ContractOpts{}
	}
	var results []ContractResult
	ops := idx.AllOperations()
	for _, opr := range ops {
		if opr == nil || opr.Operation == nil {
			continue
		}
		op := opr.Operation
		if len(opts.Tags) > 0 && !hasAnyTag(op.TagNames(), opts.Tags) {
			continue
		}
		if opts.OperationID != "" && op.OperationID != opts.OperationID {
			continue
		}
		pathParams := placeholderPathParams(idx, opr.Path, pathItemFor(idx, opr.Path), op)
		req, err := BuildRequest(ctx, idx, baseURL, opr.Path, strings.ToUpper(opr.Method), pathItemFor(idx, opr.Path), op, pathParams, nil, nil)
		if err != nil {
			results = append(results, ContractResult{
				Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
				Pass: false, Error: err.Error(),
			})
			continue
		}
		if err := ApplySecurity(idx, op, req, opts.Credentials); err != nil {
			results = append(results, ContractResult{
				Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
				Pass: false, Error: err.Error(),
			})
			continue
		}
		resp, err := client.Do(ctx, req)
		if err != nil {
			results = append(results, ContractResult{
				Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
				Pass: false, Error: err.Error(),
			})
			continue
		}
		statusCode := resp.StatusCode
		bodyBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			results = append(results, ContractResult{
				Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
				Pass: false, Status: statusCode, Error: readErr.Error(),
			})
			continue
		}
		legacyErr := ValidateResponse(ctx, idx, cloneResponse(resp, bodyBytes), op, statusCode)
		var validatorErrs []ValidationError
		if opts.ResponseValidator != nil {
			validatorErrs = opts.ResponseValidator.ValidateResponse(req, cloneResponse(resp, bodyBytes))
		}
		mergedErr, guidelineID, docURL := mergeValidationErrors(legacyErr, validatorErrs)
		pass := mergedErr == nil
		errStr := ""
		if mergedErr != nil {
			errStr = mergedErr.Error()
		}
		results = append(results, ContractResult{
			Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
			Pass: pass, Status: statusCode, Error: errStr, GuidelineID: guidelineID, DocURL: docURL,
		})
	}
	return results, nil
}

// ContractOpts filters which operations to run.
type ContractOpts struct {
	Tags        []string
	OperationID string
	// Credentials maps security scheme name (components.securitySchemes) to secret values
	// (API keys, bearer tokens, basic "user:pass", OAuth access tokens). Resolved by the host.
	Credentials       map[string]string
	ResponseValidator ResponseValidator
}

func cloneResponse(resp *http.Response, body []byte) *http.Response {
	if resp == nil {
		return nil
	}
	clone := new(http.Response)
	*clone = *resp
	clone.Header = resp.Header.Clone()
	if body != nil {
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.ContentLength = int64(len(body))
	} else {
		clone.Body = http.NoBody
		clone.ContentLength = 0
	}
	return clone
}

func mergeValidationErrors(legacyErr error, validatorErrs []ValidationError) (error, string, string) {
	var parts []string
	guidelineID := ""
	docURL := ""
	if legacyErr != nil {
		parts = append(parts, legacyErr.Error())
		guidelineID, docURL = guidelineMetadataFromError(legacyErr)
	}
	for _, err := range validatorErrs {
		parts = append(parts, err.Error())
	}
	if len(parts) == 0 {
		return nil, guidelineID, docURL
	}
	return errors.New(strings.Join(parts, "; ")), guidelineID, docURL
}

func hasAnyTag(opTags, filter []string) bool {
	for _, t := range opTags {
		for _, f := range filter {
			if t == f {
				return true
			}
		}
	}
	return false
}

func pathItemFor(idx *navigator.Index, path string) *navigator.PathItem {
	if idx == nil || idx.Document == nil || idx.Document.Paths == nil {
		return nil
	}
	return idx.Document.Paths[path]
}

func placeholderPathParams(idx *navigator.Index, path string, pathItem *navigator.PathItem, op *navigator.Operation) map[string]string {
	m := make(map[string]string)
	if pathItem != nil {
		for _, p := range pathItem.Parameters {
			p = resolveParameter(idx, p)
			if p != nil && p.In == "path" {
				if _, ok := m[p.Name]; !ok {
					m[p.Name] = paramValue(idx, p, m)
				}
			}
		}
	}
	if op != nil {
		for _, p := range op.Parameters {
			p = resolveParameter(idx, p)
			if p != nil && p.In == "path" {
				if _, ok := m[p.Name]; !ok {
					m[p.Name] = paramValue(idx, p, m)
				}
			}
		}
	}
	for _, seg := range strings.Split(path, "/") {
		if len(seg) > 2 && seg[0] == '{' && seg[len(seg)-1] == '}' {
			name := seg[1 : len(seg)-1]
			if _, ok := m[name]; !ok {
				m[name] = "0"
			}
		}
	}
	return m
}
