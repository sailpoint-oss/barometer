package barometer

import (
	"context"
	"strings"
	"time"

	"github.com/sailpoint-oss/barrelman"
	navigator "github.com/sailpoint-oss/navigator"
)

// ContractTestRule returns a barrelman Rule that runs contract tests against
// baseURL and reports failures as diagnostics. Can be used standalone with
// barrelman's engine, or wrapped by editor/CLI integrations.
func ContractTestRule(baseURL string, opts *RunOpts) barrelman.Rule {
	return barrelman.Define("barometer-contract", barrelman.RuleMeta{
		Description: "Contract tests must pass against live API",
		Severity:    barrelman.SeverityError,
		Category:    barrelman.CategoryStructure,
	}).Custom(func(idx *navigator.Index, r *barrelman.Reporter) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result, err := RunWithIndex(ctx, idx, baseURL, opts)
		if err != nil {
			if idx != nil && idx.Document != nil {
				r.At(idx.Document.Loc, "Contract test run failed: %v", err)
			}
			return
		}
		if result == nil || result.OpenAPI == nil {
			return
		}
		for _, res := range result.OpenAPI.Results {
			if !res.Pass {
				message := res.Error
				if res.GuidelineID != "" {
					message = "[#" + strings.TrimPrefix(res.GuidelineID, "sp-") + "] " + message
				}
				opRef := idx.Operations[res.OperationID]
				if opRef != nil && opRef.Operation != nil {
					r.Error(opRef.Operation.Loc, "Contract test failed: %s", message)
				} else if idx.Document != nil {
					r.At(idx.Document.Loc, "%s %s: %s", res.Method, res.Path, message)
				}
			}
		}
	}).Build()
}
