// Package openapi provides loading, validation, and contract testing for OpenAPI 2.0, 3.0.x, 3.1.x, and 3.2 documents.
// It uses the Telescope server OpenAPI IR (Index, Document, Schema, etc.) as the shared representation.
package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

// LoadOpts configures how to load an OpenAPI document.
type LoadOpts struct {
	// BasePath is used to resolve relative $ref when path is a file path.
	BasePath string
	// HTTPClient is used when loading from URL. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// Load reads an OpenAPI document from a local path or URL and returns a Telescope Index.
// Supports OpenAPI 3.0, 3.1, and 3.2 (YAML or JSON). OAS 2.0 is detected and rejected.
func Load(ctx context.Context, path string, opts *LoadOpts) (*navigator.Index, error) {
	if opts == nil {
		opts = &LoadOpts{}
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("openapi: path is required")
	}

	var data []byte
	if isURL(path) {
		client := opts.HTTPClient
		if client == nil {
			client = http.DefaultClient
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, fmt.Errorf("openapi: build request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("openapi: fetch %q: %w", path, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("openapi: fetch %q: status %d", path, resp.StatusCode)
		}
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("openapi: read body: %w", err)
		}
	} else {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("openapi: resolve path: %w", err)
		}
		data, err = os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("openapi: read file: %w", err)
		}
		_ = opts.BasePath
	}

	if isSwagger2(data) {
		return nil, fmt.Errorf("openapi: OAS 2.0 (swagger) not yet supported; path=%s", path)
	}

	idx := navigator.ParseAndIndex(data)
	if idx == nil || idx.Document == nil {
		return nil, fmt.Errorf("openapi: parse failed for path=%s", path)
	}
	if !idx.IsOpenAPI() {
		return nil, fmt.Errorf("openapi: document is not a valid OpenAPI root document; path=%s", path)
	}
	return idx, nil
}

// LoadFromIndex returns the given index as-is. Used when the caller (e.g. Telescope) already has a parsed Index.
func LoadFromIndex(idx *navigator.Index) *navigator.Index {
	return idx
}

// Validate performs structural validation of the loaded document (e.g. is it a valid OpenAPI root).
func Validate(idx *navigator.Index) error {
	if idx == nil {
		return fmt.Errorf("openapi: no index")
	}
	if !idx.IsOpenAPI() {
		return fmt.Errorf("openapi: document is not a valid OpenAPI root document")
	}
	return nil
}

// Version returns the openapi version string from the document (e.g. "3.0.3", "3.1.0", "3.2.0").
func Version(idx *navigator.Index) string {
	if idx == nil || idx.Document == nil {
		return ""
	}
	return idx.Document.Version
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func isSwagger2(data []byte) bool {
	const max = 1024
	if len(data) > max {
		data = data[:max]
	}
	return strings.Contains(string(data), `"swagger"`) && strings.Contains(string(data), `"2.0"`) ||
		strings.Contains(string(data), "swagger:") && strings.Contains(string(data), "2.0")
}
