package openapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pb33f/libopenapi"
	libopenapivalidator "github.com/pb33f/libopenapi-validator"
	validatorerrors "github.com/pb33f/libopenapi-validator/errors"
	"github.com/pb33f/libopenapi/datamodel"
)

// ResponseValidator validates a runtime HTTP response against an OpenAPI document.
type ResponseValidator interface {
	ValidateResponse(request *http.Request, response *http.Response) []ValidationError
}

type libOpenAPIResponseValidator struct {
	validator libopenapivalidator.Validator
}

// NewLibOpenAPIResponseValidator creates a response validator backed by pb33f/libopenapi-validator.
func NewLibOpenAPIResponseValidator(ctx context.Context, specPath string, httpClient *http.Client) (ResponseValidator, error) {
	specPath = strings.TrimSpace(specPath)
	if specPath == "" {
		return nil, fmt.Errorf("openapi: spec path is required")
	}
	specBytes, cfg, err := loadLibOpenAPISpec(ctx, specPath, httpClient)
	if err != nil {
		return nil, err
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(specBytes, cfg)
	if err != nil {
		return nil, fmt.Errorf("openapi: parse libopenapi document: %w", err)
	}
	validator, validatorErrs := libopenapivalidator.NewValidator(doc)
	if len(validatorErrs) > 0 {
		return nil, errors.Join(validatorErrs...)
	}
	return &libOpenAPIResponseValidator{validator: validator}, nil
}

func (v *libOpenAPIResponseValidator) ValidateResponse(request *http.Request, response *http.Response) []ValidationError {
	if v == nil || v.validator == nil || request == nil || response == nil {
		return nil
	}
	_, errs := v.validator.ValidateHttpResponse(request, response)
	errs = filterIgnorableLibOpenAPIErrors(errs)
	return convertLibOpenAPIValidationErrors(errs)
}

func filterIgnorableLibOpenAPIErrors(errs []*validatorerrors.ValidationError) []*validatorerrors.ValidationError {
	if len(errs) == 0 {
		return nil
	}
	filtered := errs[:0]
	for _, err := range errs {
		if err == nil || isIgnorableLibOpenAPIError(err) {
			continue
		}
		filtered = append(filtered, err)
	}
	return filtered
}

func isIgnorableLibOpenAPIError(err *validatorerrors.ValidationError) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(joinNonEmpty(err.Message, err.Reason))
	return strings.Contains(text, "schema render failure") && strings.Contains(text, "circular reference") ||
		strings.Contains(text, "cannot render circular reference")
}

func convertLibOpenAPIValidationErrors(errs []*validatorerrors.ValidationError) []ValidationError {
	if len(errs) == 0 {
		return nil
	}
	out := make([]ValidationError, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		if len(err.SchemaValidationErrors) > 0 {
			for _, schemaErr := range err.SchemaValidationErrors {
				if schemaErr == nil {
					continue
				}
				path := schemaErr.FieldPath
				if path == "" {
					path = err.SpecPath
				}
				if path == "" {
					path = err.RequestPath
				}
				message := schemaErr.Reason
				if message == "" {
					message = joinNonEmpty(err.Message, err.Reason)
				}
				out = append(out, ValidationError{Path: path, Message: message})
			}
			continue
		}
		path := err.SpecPath
		if path == "" {
			path = err.RequestPath
		}
		if err.ParameterName != "" {
			if path == "" {
				path = err.ParameterName
			} else {
				path += "." + err.ParameterName
			}
		}
		out = append(out, ValidationError{
			Path:    path,
			Message: joinNonEmpty(err.Message, err.Reason),
		})
	}
	return out
}

func loadLibOpenAPISpec(ctx context.Context, specPath string, httpClient *http.Client) ([]byte, *datamodel.DocumentConfiguration, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	cfg := datamodel.NewDocumentConfiguration()
	cfg.RemoteURLHandler = remoteURLHandler(ctx, httpClient)
	cfg.AllowRemoteReferences = true

	if isURL(specPath) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, specPath, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("openapi: build spec request: %w", err)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("openapi: fetch spec %q: %w", specPath, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, nil, fmt.Errorf("openapi: fetch spec %q: status %d", specPath, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("openapi: read spec body: %w", err)
		}
		parsedURL, err := url.Parse(specPath)
		if err != nil {
			return nil, nil, fmt.Errorf("openapi: parse spec URL: %w", err)
		}
		cfg.BaseURL = parsedURL
		cfg.SpecFilePath = path.Base(parsedURL.Path)
		return data, cfg, nil
	}

	abs, err := filepath.Abs(specPath)
	if err != nil {
		return nil, nil, fmt.Errorf("openapi: resolve spec path: %w", err)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, fmt.Errorf("openapi: read spec file: %w", err)
	}
	cfg.BasePath = filepath.Dir(abs)
	cfg.SpecFilePath = filepath.Base(abs)
	cfg.AllowFileReferences = true
	return data, cfg, nil
}

func remoteURLHandler(ctx context.Context, httpClient *http.Client) func(string) (*http.Response, error) {
	return func(target string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, err
		}
		return httpClient.Do(req)
	}
}

func joinNonEmpty(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, ": ")
}
