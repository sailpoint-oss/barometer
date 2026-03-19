package openapi

import (
	"fmt"
	"net/url"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

// ResolveOperationByID finds the OperationRef for the given operationId.
func ResolveOperationByID(idx *navigator.Index, operationID string) (*navigator.OperationRef, error) {
	if idx == nil {
		return nil, fmt.Errorf("openapi: no index")
	}
	if ref, ok := idx.Operations[operationID]; ok {
		return ref, nil
	}
	return nil, fmt.Errorf("openapi: operationId %q not found", operationID)
}

// ResolveOperationByPath finds the operation by path template and method.
// fragment may be a JSON Pointer like "#/paths/~1pet~1findByStatus/get" or path and method separately.
func ResolveOperationByPath(idx *navigator.Index, pathPattern, method string) (*navigator.OperationRef, error) {
	if idx == nil {
		return nil, fmt.Errorf("openapi: no index")
	}
	ops, ok := idx.OperationsByPath[pathPattern]
	if !ok {
		return nil, fmt.Errorf("openapi: path %q not found", pathPattern)
	}
	method = strings.ToLower(method)
	for i := range ops {
		if ops[i].Method == method {
			return &ops[i], nil
		}
	}
	return nil, fmt.Errorf("openapi: method %q not found for path %q", method, pathPattern)
}

// ResolveOperationByPathFragment parses a JSON Pointer fragment (e.g. #/paths/~1pet~1findByStatus/get) and resolves the operation.
func ResolveOperationByPathFragment(idx *navigator.Index, fragment string) (*navigator.OperationRef, error) {
	if !strings.HasPrefix(fragment, "#/paths/") {
		return nil, fmt.Errorf("openapi: unsupported operationPath fragment")
	}
	rest := strings.TrimPrefix(fragment, "#/paths/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("openapi: invalid operationPath")
	}
	pathEnc, method := parts[0], parts[1]
	path := strings.ReplaceAll(pathEnc, "~1", "/")
	path = strings.ReplaceAll(path, "~0", "~")
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return ResolveOperationByPath(idx, path, method)
}
