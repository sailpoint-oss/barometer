package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

// ParamOverrides can supply values for parameters (key: "path:name", "query:name", "header:name", "cookie:name").
// Used by Arazzo to inject resolved expression values.
type ParamOverrides map[string]string

func (o ParamOverrides) get(in, name string) string {
	if o == nil {
		return ""
	}
	return o[in+":"+name]
}

// BuildRequest builds an HTTP request for the given path, method, and operation
// using Navigator-backed OpenAPI types. If bodyOverride is non-nil, it is used
// as the request body instead of building from the operation's requestBody schema.
func BuildRequest(ctx context.Context, idx *navigator.Index, baseURL, pathTemplate, method string, pathItem *navigator.PathItem, op *navigator.Operation, pathParams map[string]string, overrides ParamOverrides, bodyOverride []byte) (*http.Request, error) {
	if pathItem == nil || op == nil {
		return nil, fmt.Errorf("openapi: pathItem and operation required")
	}
	baseURL = strings.TrimSuffix(baseURL, "/")
	path := pathTemplate
	for name, val := range pathParams {
		path = strings.ReplaceAll(path, "{"+name+"}", url.PathEscape(val))
	}
	re := regexp.MustCompile(`\{[^}]+\}`)
	path = re.ReplaceAllStringFunc(path, func(m string) string {
		name := m[1 : len(m)-1]
		if v, ok := pathParams[name]; ok {
			return url.PathEscape(v)
		}
		return "0"
	})
	reqURL := baseURL + path

	params := collectParams(pathItem, op, idx)
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	var headers http.Header
	for _, p := range params {
		if p == nil {
			continue
		}
		val := overrides.get(p.In, p.Name)
		if val == "" {
			val = paramValue(idx, p, pathParams)
		}
		switch p.In {
		case "path":
			// already applied
		case "query":
			q.Set(p.Name, val)
		case "header":
			if headers == nil {
				headers = make(http.Header)
			}
			headers.Set(p.Name, val)
		case "cookie":
			if headers == nil {
				headers = make(http.Header)
			}
			headers.Set("Cookie", headers.Get("Cookie")+"; "+p.Name+"="+url.QueryEscape(val))
		}
	}
	u.RawQuery = q.Encode()
	reqURL = u.String()

	body := bodyOverride
	if body == nil && op.RequestBody != nil {
		rb := resolveRequestBody(idx, op.RequestBody)
		if rb != nil {
			body, err = buildRequestBody(idx, rb)
			if err != nil {
				return nil, err
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(method), reqURL, nil)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		if req.Header.Get("Content-Type") == "" {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	for k, v := range headers {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	return req, nil
}

func collectParams(pathItem *navigator.PathItem, op *navigator.Operation, idx *navigator.Index) []*navigator.Parameter {
	var out []*navigator.Parameter
	if pathItem != nil {
		for _, p := range pathItem.Parameters {
			resolved := resolveParameter(idx, p)
			if resolved != nil {
				out = append(out, resolved)
			}
		}
	}
	if op != nil {
		for _, p := range op.Parameters {
			resolved := resolveParameter(idx, p)
			if resolved != nil {
				out = append(out, resolved)
			}
		}
	}
	return out
}

func resolveParameter(idx *navigator.Index, p *navigator.Parameter) *navigator.Parameter {
	if p == nil {
		return nil
	}
	if p.Ref == "" {
		return p
	}
	resolved, err := idx.ResolveRef(p.Ref)
	if err != nil {
		return p
	}
	if r, ok := resolved.(*navigator.Parameter); ok {
		return r
	}
	return p
}

func resolveRequestBody(idx *navigator.Index, rb *navigator.RequestBody) *navigator.RequestBody {
	if rb == nil {
		return nil
	}
	if rb.Ref == "" {
		return rb
	}
	resolved, err := idx.ResolveRef(rb.Ref)
	if err != nil {
		return rb
	}
	if r, ok := resolved.(*navigator.RequestBody); ok {
		return r
	}
	return rb
}

func paramValue(idx *navigator.Index, p *navigator.Parameter, pathParams map[string]string) string {
	if p == nil {
		return ""
	}
	if v, ok := pathParams[p.Name]; ok {
		return v
	}
	if p.Example != nil {
		return p.Example.Value
	}
	if p.Schema != nil {
		s := resolveSchema(idx, p.Schema)
		if s != nil && s.Default != nil {
			return s.Default.Value
		}
		if s != nil {
			switch s.Type {
			case "integer", "number":
				return "1"
			case "boolean":
				return "false"
			case "string":
				if strings.EqualFold(s.Format, "uuid") {
					return "550e8400-e29b-41d4-a716-446655440000"
				}
				if p.In == "path" && strings.HasSuffix(strings.ToLower(p.Name), "id") {
					return "550e8400-e29b-41d4-a716-446655440000"
				}
				return "test"
			}
		}
	}
	if p.In == "path" {
		if strings.HasSuffix(strings.ToLower(p.Name), "id") {
			return "550e8400-e29b-41d4-a716-446655440000"
		}
		return "test"
	}
	return ""
}

func resolveSchema(idx *navigator.Index, s *navigator.Schema) *navigator.Schema {
	if s == nil || s.Ref == "" {
		return s
	}
	resolved, err := idx.ResolveRef(s.Ref)
	if err != nil {
		return s
	}
	if r, ok := resolved.(*navigator.Schema); ok {
		return r
	}
	return s
}

func buildRequestBody(idx *navigator.Index, rb *navigator.RequestBody) ([]byte, error) {
	if rb == nil || rb.Content == nil {
		return nil, nil
	}
	mt := rb.Content["application/json"]
	if mt == nil {
		for _, v := range rb.Content {
			mt = v
			break
		}
	}
	if mt == nil {
		return nil, nil
	}
	if mt.Example != nil && mt.Example.Value != "" {
		return []byte(mt.Example.Value), nil
	}
	if len(mt.Examples) > 0 {
		for _, e := range mt.Examples {
			if e != nil && e.Value != nil && e.Value.Value != "" {
				return []byte(e.Value.Value), nil
			}
		}
	}
	if mt.Schema != nil {
		s := resolveSchema(idx, mt.Schema)
		if s != nil {
			return minimalJSONFromSchema(idx, s), nil
		}
	}
	return nil, nil
}

func minimalJSONFromSchema(idx *navigator.Index, s *navigator.Schema) []byte {
	if s == nil {
		return []byte("{}")
	}
	s = resolveSchema(idx, s)
	if s == nil {
		return []byte("{}")
	}
	m := make(map[string]any)
	if s.Properties != nil {
		for name, prop := range s.Properties {
			prop = resolveSchema(idx, prop)
			if prop != nil {
				m[name] = defaultForSchema(idx, prop)
			}
		}
	}
	for _, r := range s.Required {
		if _, ok := m[r]; !ok {
			m[r] = nil
		}
	}
	b, _ := json.Marshal(m)
	return b
}

func defaultForSchema(idx *navigator.Index, s *navigator.Schema) any {
	if s == nil {
		return nil
	}
	s = resolveSchema(idx, s)
	if s == nil {
		return nil
	}
	if s.Default != nil {
		return s.Default.Value
	}
	switch s.Type {
	case "string":
		return ""
	case "integer", "number":
		return 0
	case "boolean":
		return false
	case "array":
		return []any{}
	case "object":
		return map[string]any{}
	}
	return nil
}
