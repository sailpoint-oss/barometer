package openapi

import (
	"context"
	"fmt"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
	"github.com/sailpoint-oss/barometer/internal/runner"
)

// ContractResult is the result of a single operation contract check.
type ContractResult struct {
	Path        string
	Method      string
	OperationID string
	Pass        bool
	Status      int
	Error       string
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
		resp, err := client.Do(ctx, req)
		if err != nil {
			results = append(results, ContractResult{
				Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
				Pass: false, Error: err.Error(),
			})
			continue
		}
		statusCode := resp.StatusCode
		err = ValidateResponse(ctx, idx, resp, op, statusCode)
		resp.Body.Close()
		pass := err == nil
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		results = append(results, ContractResult{
			Path: opr.Path, Method: opr.Method, OperationID: op.OperationID,
			Pass: pass, Status: statusCode, Error: errStr,
		})
	}
	return results, nil
}

// ContractOpts filters which operations to run.
type ContractOpts struct {
	Tags        []string
	OperationID string
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
