package openapi

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	navigator "github.com/sailpoint-oss/navigator"
)

// effectiveOperationSecurity returns the security requirements for an operation per OAS 3:
// explicit empty security: [] disables auth; omitted operation security inherits document security.
func effectiveOperationSecurity(doc *navigator.Document, op *navigator.Operation) []navigator.SecurityRequirement {
	if op != nil && op.Security != nil {
		if len(op.Security) == 0 {
			return nil
		}
		return op.Security
	}
	if doc != nil {
		return doc.Security
	}
	return nil
}

// pickSecurityRequirement chooses the first OR-branch where every scheme has a non-empty credential.
func pickSecurityRequirement(reqs []navigator.SecurityRequirement, creds map[string]string) (*navigator.SecurityRequirement, error) {
	if len(reqs) == 0 {
		return nil, nil
	}
	for i := range reqs {
		sr := &reqs[i]
		ok := true
		for _, e := range sr.Entries {
			if strings.TrimSpace(creds[e.Name]) == "" {
				ok = false
				break
			}
		}
		if ok {
			return sr, nil
		}
	}
	var missing []string
	for _, e := range reqs[0].Entries {
		missing = append(missing, e.Name)
	}
	return nil, fmt.Errorf("no matching credentials for security requirement (need all of: %v); set contractTests.credentials.<schemeName> in .telescope.yaml or non-empty env vars for each scheme", missing)
}

// ApplySecurity applies OpenAPI security schemes to req using resolved credential strings
// keyed by security scheme name (same keys as components.securitySchemes).
func ApplySecurity(idx *navigator.Index, op *navigator.Operation, req *http.Request, creds map[string]string) error {
	if idx == nil || req == nil {
		return nil
	}
	if creds == nil {
		creds = map[string]string{}
	}
	reqs := effectiveOperationSecurity(idx.Document, op)
	if len(reqs) == 0 {
		return nil
	}
	sr, err := pickSecurityRequirement(reqs, creds)
	if err != nil {
		return err
	}
	if sr == nil {
		return fmt.Errorf("credentials required for security but none provided; configure contractTests.credentials for schemes in components.securitySchemes")
	}
	for _, e := range sr.Entries {
		val := strings.TrimSpace(creds[e.Name])
		if val == "" {
			return fmt.Errorf("missing credential for security scheme %q (set contractTests.credentials.%[1]s or matching *Env variables)", e.Name)
		}
		ss := lookupSecurityScheme(idx, e.Name)
		if ss == nil {
			return fmt.Errorf("unknown security scheme %q", e.Name)
		}
		if err := applyScheme(ss, val, req); err != nil {
			return err
		}
	}
	return nil
}

func lookupSecurityScheme(idx *navigator.Index, name string) *navigator.SecurityScheme {
	if idx == nil {
		return nil
	}
	if ss, ok := idx.SecuritySchemes[name]; ok && ss != nil {
		return ss
	}
	if idx.Document != nil && idx.Document.Components != nil {
		if ss, ok := idx.Document.Components.SecuritySchemes[name]; ok {
			return ss
		}
	}
	return nil
}

func applyScheme(ss *navigator.SecurityScheme, value string, req *http.Request) error {
	t := strings.ToLower(strings.TrimSpace(ss.Type))
	switch t {
	case "apikey":
		switch strings.ToLower(ss.In) {
		case "header":
			req.Header.Set(ss.Name, value)
		case "query":
			q := req.URL.Query()
			q.Set(ss.Name, value)
			req.URL.RawQuery = q.Encode()
		case "cookie":
			req.Header.Add("Cookie", ss.Name+"="+value)
		default:
			return fmt.Errorf("apiKey scheme %q: unsupported in %q", ss.Name, ss.In)
		}
		return nil
	case "http":
		switch strings.ToLower(ss.Scheme) {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+value)
		case "basic":
			// value is raw "user:password" or already base64 — accept both
			if strings.Contains(value, ":") {
				req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(value)))
			} else {
				req.Header.Set("Authorization", "Basic "+value)
			}
		default:
			return fmt.Errorf("http scheme %q not supported", ss.Scheme)
		}
		return nil
	case "oauth2", "openidconnect":
		// Access token from credentials (interactive OAuth obtains token out-of-band)
		req.Header.Set("Authorization", "Bearer "+value)
		return nil
	default:
		return fmt.Errorf("security scheme type %q not supported", ss.Type)
	}
}
