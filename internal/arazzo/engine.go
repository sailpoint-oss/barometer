package arazzo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
	"github.com/sailpoint-oss/barometer/internal/openapi"
	"github.com/sailpoint-oss/barometer/internal/runner"
)

// RunWorkflow executes a single workflow with the given inputs and base URL.
func (d *Doc) RunWorkflow(ctx context.Context, workflowID, baseURL string, inputs map[string]any, client *runner.Client) (map[string]any, error) {
	var w *Workflow
	for i := range d.Workflows {
		if d.Workflows[i].WorkflowID == workflowID {
			w = &d.Workflows[i]
			break
		}
	}
	if w == nil {
		return nil, fmt.Errorf("workflow %q not found", workflowID)
	}
	for _, depID := range w.DependsOn {
		depOut, err := d.RunWorkflow(ctx, depID, baseURL, inputs, client)
		if err != nil {
			return nil, fmt.Errorf("dependency workflow %q: %w", depID, err)
		}
		for k, v := range depOut {
			if inputs == nil {
				inputs = make(map[string]any)
			}
			inputs[k] = v
		}
	}
	sources := make(map[string]*navigator.Index)
	for _, sd := range d.SourceDescriptions {
		if sd.Type != "openapi" {
			continue
		}
		idx, err := openapi.Load(ctx, sd.URL, nil)
		if err != nil {
			return nil, fmt.Errorf("load source %q: %w", sd.Name, err)
		}
		sources[sd.Name] = idx
	}
	sourceDescMap := make(map[string]any)
	for _, sd := range d.SourceDescriptions {
		sourceDescMap[sd.Name] = map[string]any{"url": sd.URL, "type": sd.Type}
	}
	ctx2 := &RuntimeContext{
		Inputs:             inputs,
		Steps:              make(map[string]StepOutputs),
		SourceDescriptions: sourceDescMap,
	}
	stepIndex := 0
	for stepIndex < len(w.Steps) {
		step := w.Steps[stepIndex]
		ctx2.Response = nil
		stepOut, err := d.runStep(ctx, step, w, baseURL, sources, ctx2, client)
		if err != nil {
			nextIdx, stop, retryErr := d.evalFailureActions(ctx, step, w, stepIndex, baseURL, sources, ctx2, client, err)
			if retryErr != nil {
				return nil, retryErr
			}
			if stop {
				return nil, err
			}
			if nextIdx >= 0 {
				stepIndex = nextIdx
				continue
			}
			return nil, fmt.Errorf("step %q: %w", step.StepID, err)
		}
		ctx2.Steps[step.StepID] = stepOut
		stepIndex++
	}
	outputs := make(map[string]any)
	for name, expr := range w.Outputs {
		v, err := ctx2.Resolve(expr)
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", name, err)
		}
		outputs[name] = v
	}
	return outputs, nil
}

func (d *Doc) evalFailureActions(ctx context.Context, step Step, w *Workflow, currentIdx int, baseURL string, sources map[string]*navigator.Index, ctx2 *RuntimeContext, client *runner.Client, stepErr error) (nextIdx int, stop bool, err error) {
	nextIdx = -1
	if len(step.OnFailure) == 0 {
		return -1, true, nil
	}
	for _, a := range step.OnFailure {
		am := toMapAny(a)
		if am == nil {
			continue
		}
		typ, _ := am["type"].(string)
		switch typ {
		case "end":
			return -1, true, nil
		case "goto":
			stepId := fmt.Sprint(am["stepId"])
			if stepId != "" && stepId != "<nil>" {
				for i, s := range w.Steps {
					if s.StepID == stepId {
						return i, false, nil
					}
				}
			}
		case "retry":
			return currentIdx, false, nil
		}
	}
	return -1, true, nil
}

func (d *Doc) runStep(ctx context.Context, step Step, w *Workflow, baseURL string, sources map[string]*navigator.Index, ctx2 *RuntimeContext, client *runner.Client) (StepOutputs, error) {
	var path, method string
	var pathItem *navigator.PathItem
	var op *navigator.Operation
	var idx *navigator.Index
	if step.OperationID != "" {
		for _, sd := range d.SourceDescriptions {
			if sd.Type != "openapi" {
				continue
			}
			idx = sources[sd.Name]
			if idx == nil {
				continue
			}
			ref, err := openapi.ResolveOperationByID(idx, step.OperationID)
			if err == nil {
				path = ref.Path
				method = ref.Method
				pathItem = pathItemFor(idx, ref.Path)
				op = ref.Operation
				break
			}
		}
	} else if step.OperationPath != "" {
		fragment, docName := parseOperationPath(step.OperationPath)
		if fragment == "" {
			return nil, fmt.Errorf("invalid operationPath: %q", step.OperationPath)
		}
		if docName != "" {
			idx = sources[docName]
		}
		if idx == nil {
			for _, i := range sources {
				idx = i
				break
			}
		}
		if idx == nil {
			return nil, fmt.Errorf("no OpenAPI source for operationPath")
		}
		ref, err := openapi.ResolveOperationByPathFragment(idx, fragment)
		if err != nil {
			return nil, err
		}
		path = ref.Path
		method = ref.Method
		pathItem = pathItemFor(idx, ref.Path)
		op = ref.Operation
	} else if step.WorkflowID != "" {
		subInputs := make(map[string]any)
		for _, p := range step.Parameters {
			val, _ := ctx2.Resolve(p.Value)
			subInputs[p.Name] = val
		}
		if len(subInputs) == 0 && ctx2.Inputs != nil {
			subInputs = ctx2.Inputs
		}
		out, err := d.RunWorkflow(ctx, step.WorkflowID, baseURL, subInputs, client)
		if err != nil {
			return nil, err
		}
		outputs := make(StepOutputs)
		for k, v := range out {
			outputs[k] = v
		}
		return outputs, nil
	} else {
		return nil, fmt.Errorf("step must have operationId, operationPath, or workflowId")
	}
	if path == "" || op == nil || idx == nil {
		return nil, fmt.Errorf("could not resolve operation")
	}
	// Merge workflow-level parameters with step parameters (step overrides).
	combinedParams := make([]Parameter, 0, len(w.Parameters)+len(step.Parameters))
	combinedParams = append(combinedParams, w.Parameters...)
	combinedParams = append(combinedParams, step.Parameters...)
	overrides := buildParamOverrides(combinedParams, ctx2)
	pathParams := extractPathParams(combinedParams, path, ctx2)

	var bodyOverride []byte
	if step.RequestBody != nil {
		var bodyErr error
		bodyOverride, bodyErr = resolveStepRequestBody(step.RequestBody, ctx2)
		if bodyErr != nil {
			return nil, fmt.Errorf("requestBody: %w", bodyErr)
		}
	}

	req, err := openapi.BuildRequest(ctx, idx, baseURL, path, method, pathItem, op, pathParams, overrides, bodyOverride)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(ctx, req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var bodyVal any
	if len(body) > 0 {
		_ = json.Unmarshal(body, &bodyVal)
	}
	ctx2.Response = &StepResponse{
		StatusCode: resp.StatusCode,
		Header:     flattenHeader(resp.Header),
		Body:       bodyVal,
	}
	ok, err := ctx2.EvalSuccessCriteria(step.SuccessCriteria)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("successCriteria not satisfied (status %d)", resp.StatusCode)
	}
	outputs := make(StepOutputs)
	for name, expr := range step.Outputs {
		v, err := ctx2.Resolve(expr)
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", name, err)
		}
		outputs[name] = v
	}
	return outputs, nil
}

func pathItemFor(idx *navigator.Index, path string) *navigator.PathItem {
	if idx == nil || idx.Document == nil || idx.Document.Paths == nil {
		return nil
	}
	return idx.Document.Paths[path]
}

func parseOperationPath(s string) (fragment, sourceName string) {
	i := strings.Index(s, "#")
	if i < 0 {
		return "", ""
	}
	fragment = s[i:]
	expr := strings.TrimSpace(s[:i])
	expr = strings.TrimPrefix(expr, "{$")
	expr = strings.TrimSuffix(expr, "}")
	parts := strings.Split(expr, ".")
	if len(parts) >= 2 {
		sourceName = parts[1]
	}
	return fragment, sourceName
}

// resolveStepRequestBody resolves the step requestBody (contentType + payload) and returns the body bytes.
func resolveStepRequestBody(requestBody any, ctx *RuntimeContext) ([]byte, error) {
	m := toMapAny(requestBody)
	if m == nil {
		return nil, nil
	}
	payload, has := m["payload"]
	if !has {
		return nil, nil
	}
	// Resolve payload: if map/slice, resolve any $ expressions in values recursively; if string expression, resolve.
	resolved, err := resolvePayloadValue(payload, ctx)
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, nil
	}
	// If already raw string (e.g. JSON), use as-is
	if b, ok := resolved.(string); ok && (strings.HasPrefix(strings.TrimSpace(b), "{") || strings.HasPrefix(strings.TrimSpace(b), "[")) {
		return []byte(b), nil
	}
	return json.Marshal(resolved)
}

func resolvePayloadValue(v any, ctx *RuntimeContext) (any, error) {
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
		if strings.HasPrefix(strings.TrimSpace(s), "$") {
			return ctx.Resolve(s)
		}
		return s, nil
	}
	if m := toMapAny(v); m != nil {
		out := make(map[string]any)
		for k, val := range m {
			res, err := resolvePayloadValue(val, ctx)
			if err != nil {
				return nil, err
			}
			out[k] = res
		}
		return out, nil
	}
	if sl, ok := v.([]interface{}); ok {
		out := make([]interface{}, len(sl))
		for i, val := range sl {
			res, err := resolvePayloadValue(val, ctx)
			if err != nil {
				return nil, err
			}
			out[i] = res
		}
		return out, nil
	}
	return v, nil
}

func buildParamOverrides(params []Parameter, ctx *RuntimeContext) openapi.ParamOverrides {
	out := make(openapi.ParamOverrides)
	for _, p := range params {
		val, err := ctx.ResolveString(p.Value)
		if err != nil {
			continue
		}
		in := p.In
		if in == "" {
			in = "query"
		}
		out[in+":"+p.Name] = val
	}
	return out
}

func extractPathParams(params []Parameter, path string, ctx *RuntimeContext) map[string]string {
	m := make(map[string]string)
	for _, p := range params {
		if p.In != "path" {
			continue
		}
		val, _ := ctx.ResolveString(p.Value)
		m[p.Name] = val
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

func toMapAny(a any) map[string]any {
	if m, ok := a.(map[string]any); ok {
		return m
	}
	if m, ok := a.(map[interface{}]interface{}); ok {
		out := make(map[string]any)
		for k, v := range m {
			if s, ok := k.(string); ok {
				out[s] = v
			}
		}
		return out
	}
	return nil
}

func flattenHeader(h http.Header) map[string]string {
	m := make(map[string]string)
	for k, v := range h {
		if len(v) > 0 {
			m[k] = v[0]
		}
	}
	return m
}
