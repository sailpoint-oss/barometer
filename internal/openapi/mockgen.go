package openapi

import (
	"encoding/json"
	"math"
	"sort"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
	"gopkg.in/yaml.v3"
)

const maxMockSchemaDepth = 32

type requestBodyMock struct {
	Body        []byte
	ContentType string
}

func mockRequestBody(idx *navigator.Index, rb *navigator.RequestBody) (*requestBodyMock, error) {
	if rb == nil || rb.Content == nil {
		return nil, nil
	}
	contentType, mediaType := selectRequestMediaType(rb.Content)
	if mediaType == nil {
		return nil, nil
	}
	if body, ok := mediaTypeExampleBytes(contentType, mediaType); ok {
		return &requestBodyMock{Body: body, ContentType: contentType}, nil
	}
	if !isJSONLikeContentType(contentType) || mediaType.Schema == nil {
		return nil, nil
	}
	value := mockValueFromSchema(idx, mediaType.Schema, 0)
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return &requestBodyMock{Body: body, ContentType: contentType}, nil
}

func selectRequestMediaType(content map[string]*navigator.MediaType) (string, *navigator.MediaType) {
	if mt := content["application/json"]; mt != nil {
		return "application/json", mt
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if isJSONLikeContentType(key) {
			return key, content[key]
		}
	}
	for _, key := range keys {
		return key, content[key]
	}
	return "", nil
}

func mediaTypeExampleBytes(contentType string, mediaType *navigator.MediaType) ([]byte, bool) {
	if mediaType == nil {
		return nil, false
	}
	if body, ok := nodeBytesForContentType(contentType, mediaType.Example); ok {
		return body, true
	}
	if len(mediaType.Examples) == 0 {
		return nil, false
	}
	keys := make([]string, 0, len(mediaType.Examples))
	for key := range mediaType.Examples {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if body, ok := exampleBytesForContentType(contentType, mediaType.Examples[key]); ok {
			return body, true
		}
	}
	return nil, false
}

func exampleBytesForContentType(contentType string, example *navigator.Example) ([]byte, bool) {
	if example == nil {
		return nil, false
	}
	return nodeBytesForContentType(contentType, example.Value)
}

func nodeBytesForContentType(contentType string, node *navigator.Node) ([]byte, bool) {
	if node == nil {
		return nil, false
	}
	value := strings.TrimSpace(node.Value)
	if value == "" {
		return nil, false
	}
	if !isJSONLikeContentType(contentType) {
		return []byte(value), true
	}
	parsed, ok := parseNodeValue(node)
	if !ok {
		return []byte(value), true
	}
	body, err := json.Marshal(parsed)
	if err != nil {
		return nil, false
	}
	return body, true
}

func mockValueFromSchema(idx *navigator.Index, schema *navigator.Schema, depth int) any {
	if depth > maxMockSchemaDepth {
		return nil
	}
	schema = resolveSchema(idx, schema)
	if schema == nil {
		return nil
	}
	if example, ok := preferredSchemaExample(schema); ok {
		return example
	}
	if len(schema.OneOf) > 0 {
		return mockValueFromSchema(idx, schema.OneOf[0], depth+1)
	}
	if len(schema.AnyOf) > 0 {
		return mockValueFromSchema(idx, schema.AnyOf[0], depth+1)
	}

	switch schema.Type {
	case "object", "":
		return mockObjectFromSchema(idx, schema, depth)
	case "array":
		return mockArrayFromSchema(idx, schema, depth)
	case "string":
		return mockStringFromSchema(schema)
	case "integer":
		return int(mockNumberFromSchema(schema, true))
	case "number":
		return mockNumberFromSchema(schema, false)
	case "boolean":
		return false
	default:
		return nil
	}
}

func preferredSchemaExample(schema *navigator.Schema) (any, bool) {
	if schema == nil {
		return nil, false
	}
	if schema.Example != nil {
		if value, ok := parseNodeValue(schema.Example); ok {
			return value, true
		}
	}
	if schema.Default != nil {
		if value, ok := parseNodeValue(schema.Default); ok {
			return value, true
		}
	}
	if len(schema.Enum) > 0 {
		return schema.Enum[0], true
	}
	return nil, false
}

func mockObjectFromSchema(idx *navigator.Index, schema *navigator.Schema, depth int) any {
	out := make(map[string]any)
	if len(schema.AllOf) > 0 {
		for _, sub := range schema.AllOf {
			if obj, ok := mockValueFromSchema(idx, sub, depth+1).(map[string]any); ok {
				for key, value := range obj {
					out[key] = value
				}
			}
		}
	}
	if len(schema.Properties) > 0 {
		names := make([]string, 0, len(schema.Properties))
		for name := range schema.Properties {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			out[name] = mockValueFromSchema(idx, schema.Properties[name], depth+1)
		}
	}
	for _, required := range schema.Required {
		if _, ok := out[required]; ok {
			continue
		}
		if schema.AdditionalProperties != nil {
			out[required] = mockValueFromSchema(idx, schema.AdditionalProperties, depth+1)
			continue
		}
		out[required] = ""
	}
	if len(out) == 0 {
		return map[string]any{}
	}
	return out
}

func mockArrayFromSchema(idx *navigator.Index, schema *navigator.Schema, depth int) any {
	count := 1
	if schema.MinItems != nil && *schema.MinItems > count {
		count = *schema.MinItems
	}
	if count > 3 {
		count = 3
	}
	items := make([]any, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, mockValueFromSchema(idx, schema.Items, depth+1))
	}
	return items
}

func mockStringFromSchema(schema *navigator.Schema) string {
	value := "test"
	switch strings.ToLower(schema.Format) {
	case "uuid":
		value = "550e8400-e29b-41d4-a716-446655440000"
	case "email":
		value = "user@example.com"
	case "date":
		value = "2026-01-01"
	case "date-time":
		value = "2026-01-01T00:00:00Z"
	case "time":
		value = "00:00:00Z"
	case "uri":
		value = "https://example.com"
	case "hostname":
		value = "example.com"
	case "ipv4":
		value = "127.0.0.1"
	case "ipv6":
		value = "2001:db8::1"
	}
	if schema.MinLength != nil && *schema.MinLength > len(value) {
		value += strings.Repeat("x", *schema.MinLength-len(value))
	}
	if schema.MaxLength != nil && *schema.MaxLength < len(value) {
		value = value[:*schema.MaxLength]
	}
	return value
}

func mockNumberFromSchema(schema *navigator.Schema, integer bool) float64 {
	value := 1.0
	if schema.Minimum != nil {
		value = *schema.Minimum
	}
	if schema.ExclusiveMinimum != nil && value <= *schema.ExclusiveMinimum {
		if integer {
			value = math.Floor(*schema.ExclusiveMinimum) + 1
		} else {
			value = *schema.ExclusiveMinimum + 0.1
		}
	}
	if schema.Maximum != nil && value > *schema.Maximum {
		value = *schema.Maximum
	}
	if schema.ExclusiveMaximum != nil && value >= *schema.ExclusiveMaximum {
		if integer {
			value = math.Ceil(*schema.ExclusiveMaximum) - 1
		} else {
			value = *schema.ExclusiveMaximum - 0.1
		}
	}
	if integer {
		value = math.Round(value)
	}
	return value
}

func parseNodeValue(node *navigator.Node) (any, bool) {
	if node == nil {
		return nil, false
	}
	raw := strings.TrimSpace(node.Value)
	if raw == "" {
		return nil, false
	}
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err == nil {
		return value, true
	}
	if err := yaml.Unmarshal([]byte(raw), &value); err == nil {
		return value, true
	}
	return raw, true
}

func isJSONLikeContentType(contentType string) bool {
	base := strings.TrimSpace(strings.Split(contentType, ";")[0])
	return strings.EqualFold(base, "application/json") || strings.HasSuffix(strings.ToLower(base), "+json")
}
