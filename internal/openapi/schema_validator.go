package openapi

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

const maxSchemaDepth = 64

// ValidationError describes a single schema validation failure.
type ValidationError struct {
	Path    string // JSON path or description
	Message string
}

func (e ValidationError) Error() string {
	if e.Path != "" {
		return e.Path + ": " + e.Message
	}
	return e.Message
}

// ValidateJSON validates the value against the schema using the index for $ref resolution.
// Returns a slice of validation errors (empty if valid).
func ValidateJSON(idx *navigator.Index, schema *navigator.Schema, value any) []ValidationError {
	if idx == nil || schema == nil {
		return nil
	}
	var errs []ValidationError
	visited := make(map[string]bool)
	validateSchema(idx, schema, value, "", 0, visited, &errs)
	return errs
}

func validateSchema(idx *navigator.Index, schema *navigator.Schema, value any, path string, depth int, visited map[string]bool, errs *[]ValidationError) {
	if depth > maxSchemaDepth {
		*errs = append(*errs, ValidationError{Path: path, Message: "max schema depth exceeded (possible circular ref)"})
		return
	}
	if schema.Ref != "" {
		resolved, err := idx.ResolveRef(schema.Ref)
		if err != nil {
			*errs = append(*errs, ValidationError{Path: path, Message: "resolve $ref: " + err.Error()})
			return
		}
		s, ok := resolved.(*navigator.Schema)
		if !ok {
			*errs = append(*errs, ValidationError{Path: path, Message: "$ref does not resolve to schema"})
			return
		}
		if visited[schema.Ref] {
			return
		}
		visited[schema.Ref] = true
		defer delete(visited, schema.Ref)
		validateSchema(idx, s, value, path, depth+1, visited, errs)
		return
	}

	// Nullable
	if value == nil {
		if schema.Nullable {
			return
		}
		*errs = append(*errs, ValidationError{Path: path, Message: "value is null but schema does not allow null"})
		return
	}

	// Composition
	if len(schema.AllOf) > 0 {
		for i, sub := range schema.AllOf {
			validateSchema(idx, sub, value, path+fmt.Sprintf(".allOf[%d]", i), depth+1, copyVisited(visited), errs)
		}
		return
	}
	if len(schema.AnyOf) > 0 {
		var before int
		for i, sub := range schema.AnyOf {
			before = len(*errs)
			validateSchema(idx, sub, value, path+fmt.Sprintf(".anyOf[%d]", i), depth+1, copyVisited(visited), errs)
			if len(*errs) == before {
				return
			}
		}
		*errs = append(*errs, ValidationError{Path: path, Message: "value does not match any of the anyOf schemas"})
		return
	}
	if len(schema.OneOf) > 0 {
		if schema.Discriminator != nil && schema.Discriminator.PropertyName != "" {
			validateOneOfWithDiscriminator(idx, schema, value, path, depth, visited, errs)
			return
		}
		matched := 0
		for i, sub := range schema.OneOf {
			before := len(*errs)
			validateSchema(idx, sub, value, path+fmt.Sprintf(".oneOf[%d]", i), depth+1, copyVisited(visited), errs)
			if len(*errs) == before {
				matched++
			}
		}
		if matched != 1 {
			*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("value must match exactly one oneOf schema (matched %d)", matched)})
		}
		return
	}
	if schema.Not != nil {
		before := len(*errs)
		validateSchema(idx, schema.Not, value, path+".not", depth+1, copyVisited(visited), errs)
		if len(*errs) > before {
			*errs = (*errs)[:before]
			return
		}
		*errs = append(*errs, ValidationError{Path: path, Message: "value must not match the not schema"})
		return
	}

	// Discriminator: select oneOf/anyOf branch by property (handled above if oneOf/anyOf present; discriminator just hints which branch)
	// Type-based validation
	switch schema.Type {
	case "string":
		validateString(schema, value, path, errs)
	case "integer":
		validateInteger(schema, value, path, errs)
	case "number":
		validateNumber(schema, value, path, errs)
	case "boolean":
		if _, ok := value.(bool); !ok {
			*errs = append(*errs, ValidationError{Path: path, Message: "expected boolean"})
		}
	case "array":
		validateArray(idx, schema, value, path, depth, visited, errs)
	case "object":
		validateObject(idx, schema, value, path, depth, visited, errs)
	case "":
		// No type constraint - allow any
	default:
		*errs = append(*errs, ValidationError{Path: path, Message: "unsupported type: " + schema.Type})
	}
}

func copyVisited(m map[string]bool) map[string]bool {
	out := make(map[string]bool, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func validateString(schema *navigator.Schema, value any, path string, errs *[]ValidationError) {
	s, ok := value.(string)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path, Message: "expected string"})
		return
	}
	if len(schema.Enum) > 0 {
		found := false
		for _, e := range schema.Enum {
			if e == s {
				found = true
				break
			}
		}
		if !found {
			*errs = append(*errs, ValidationError{Path: path, Message: "value not in enum"})
		}
	}
	if schema.MinLength != nil && len(s) < *schema.MinLength {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("string length %d < minLength %d", len(s), *schema.MinLength)})
	}
	if schema.MaxLength != nil && len(s) > *schema.MaxLength {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("string length %d > maxLength %d", len(s), *schema.MaxLength)})
	}
	if schema.Pattern != "" {
		re, err := regexp.Compile(schema.Pattern)
		if err == nil && !re.MatchString(s) {
			*errs = append(*errs, ValidationError{Path: path, Message: "string does not match pattern"})
		}
	}
	if schema.Format != "" && !validateFormat(schema.Format, s) {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("string does not match format %q", schema.Format)})
	}
}

// validateFormat returns true if the string matches the OpenAPI string format.
func validateFormat(format, s string) bool {
	switch format {
	case "date-time":
		return regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?$`).MatchString(s)
	case "date":
		return regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`).MatchString(s)
	case "time":
		return regexp.MustCompile(`^\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?$`).MatchString(s)
	case "email":
		return len(s) > 0 && len(s) < 255 && strings.Contains(s, "@") && regexp.MustCompile(`^[^@]+@[^@]+\.[^@]+$`).MatchString(s)
	case "uuid":
		return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`).MatchString(s)
	case "uri":
		u, err := url.Parse(s)
		return err == nil && u.Scheme != "" && u.Host != ""
	case "uri-reference":
		_, err := url.Parse(s)
		return err == nil
	case "ipv4":
		return net.ParseIP(s) != nil && strings.Contains(s, ".")
	case "ipv6":
		ip := net.ParseIP(s)
		return ip != nil && ip.To4() == nil
	case "hostname":
		return len(s) > 0 && len(s) < 254 && !strings.Contains(s, " ")
	default:
		return true
	}
}

func validateOneOfWithDiscriminator(idx *navigator.Index, schema *navigator.Schema, value any, path string, depth int, visited map[string]bool, errs *[]ValidationError) {
	obj, ok := value.(map[string]any)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path, Message: "discriminator requires object value"})
		return
	}
	propName := schema.Discriminator.PropertyName
	discVal, has := obj[propName]
	if !has {
		*errs = append(*errs, ValidationError{Path: path + "." + propName, Message: "discriminator property missing"})
		return
	}
	discStr, ok := discVal.(string)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path + "." + propName, Message: "discriminator property must be string"})
		return
	}
	var targetRef string
	if schema.Discriminator.Mapping != nil {
		targetRef = schema.Discriminator.Mapping[discStr]
	}
	if targetRef == "" {
		targetRef = "#/components/schemas/" + discStr
	}
	targetName := refToSchemaName(targetRef)
	var branchRef string
	for _, oneOfSchema := range schema.OneOf {
		if oneOfSchema == nil {
			continue
		}
		if oneOfSchema.Ref == targetRef {
			branchRef = oneOfSchema.Ref
			break
		}
		if targetName != "" && refToSchemaName(oneOfSchema.Ref) == targetName {
			branchRef = oneOfSchema.Ref
			break
		}
	}
	if branchRef == "" {
		*errs = append(*errs, ValidationError{Path: path, Message: "no oneOf branch matched discriminator " + discStr})
		return
	}
	resolved, err := idx.ResolveRef(branchRef)
	if err != nil {
		*errs = append(*errs, ValidationError{Path: path, Message: "resolve oneOf $ref: " + err.Error()})
		return
	}
	res, ok := resolved.(*navigator.Schema)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path, Message: "oneOf $ref does not resolve to schema"})
		return
	}
	validateSchema(idx, res, value, path+".oneOf", depth+1, copyVisited(visited), errs)
}

func refToSchemaName(ref string) string {
	if i := strings.LastIndex(ref, "/"); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

func validateInteger(schema *navigator.Schema, value any, path string, errs *[]ValidationError) {
	var f float64
	switch v := value.(type) {
	case float64:
		f = v
		if f != float64(int64(f)) {
			*errs = append(*errs, ValidationError{Path: path, Message: "expected integer (no fractional part)"})
			return
		}
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	default:
		*errs = append(*errs, ValidationError{Path: path, Message: "expected integer"})
		return
	}
	validateNumberConstraints(schema, f, path, errs)
}

func validateNumber(schema *navigator.Schema, value any, path string, errs *[]ValidationError) {
	var f float64
	switch v := value.(type) {
	case float64:
		f = v
	case int:
		f = float64(v)
	case int64:
		f = float64(v)
	default:
		*errs = append(*errs, ValidationError{Path: path, Message: "expected number"})
		return
	}
	validateNumberConstraints(schema, f, path, errs)
}

func validateNumberConstraints(schema *navigator.Schema, f float64, path string, errs *[]ValidationError) {
	if schema.Minimum != nil && f < *schema.Minimum {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("value %g < minimum %g", f, *schema.Minimum)})
	}
	if schema.Maximum != nil && f > *schema.Maximum {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("value %g > maximum %g", f, *schema.Maximum)})
	}
	if schema.ExclusiveMinimum != nil && f <= *schema.ExclusiveMinimum {
		*errs = append(*errs, ValidationError{Path: path, Message: "value must be > exclusiveMinimum"})
	}
	if schema.ExclusiveMaximum != nil && f >= *schema.ExclusiveMaximum {
		*errs = append(*errs, ValidationError{Path: path, Message: "value must be < exclusiveMaximum"})
	}
}

func validateArray(idx *navigator.Index, schema *navigator.Schema, value any, path string, depth int, visited map[string]bool, errs *[]ValidationError) {
	arr, ok := value.([]any)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path, Message: "expected array"})
		return
	}
	if schema.MinItems != nil && len(arr) < *schema.MinItems {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("array length %d < minItems %d", len(arr), *schema.MinItems)})
	}
	if schema.MaxItems != nil && len(arr) > *schema.MaxItems {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("array length %d > maxItems %d", len(arr), *schema.MaxItems)})
	}
	if schema.Items != nil {
		for i, item := range arr {
			validateSchema(idx, schema.Items, item, path+"["+strconv.Itoa(i)+"]", depth+1, copyVisited(visited), errs)
		}
	}
}

func validateObject(idx *navigator.Index, schema *navigator.Schema, value any, path string, depth int, visited map[string]bool, errs *[]ValidationError) {
	obj, ok := value.(map[string]any)
	if !ok {
		*errs = append(*errs, ValidationError{Path: path, Message: "expected object"})
		return
	}
	for _, r := range schema.Required {
		if _, ok := obj[r]; !ok {
			*errs = append(*errs, ValidationError{Path: path + "." + r, Message: "required property missing"})
		}
	}
	if schema.Properties != nil {
		for name, prop := range schema.Properties {
			if v, has := obj[name]; has && prop != nil {
				validateSchema(idx, prop, v, path+"."+name, depth+1, copyVisited(visited), errs)
			}
		}
	}
	if schema.AdditionalPropertiesFalse && schema.AdditionalProperties == nil {
		for k := range obj {
			if schema.Properties != nil && schema.Properties[k] != nil {
				continue
			}
			found := false
			for _, r := range schema.Required {
				if r == k {
					found = true
					break
				}
			}
			if !found {
				*errs = append(*errs, ValidationError{Path: path + "." + k, Message: "additional property not allowed"})
			}
		}
	}
	if schema.AdditionalProperties != nil {
		for k, v := range obj {
			if schema.Properties != nil && schema.Properties[k] != nil {
				continue
			}
			validateSchema(idx, schema.AdditionalProperties, v, path+"."+k, depth+1, copyVisited(visited), errs)
		}
	}
	if schema.MaxProperties != nil && len(obj) > *schema.MaxProperties {
		*errs = append(*errs, ValidationError{Path: path, Message: fmt.Sprintf("object has %d properties > maxProperties %d", len(obj), *schema.MaxProperties)})
	}
}
