// Package barometer provides the public API for OpenAPI and Arazzo contract testing.
// It supports both synchronous Run and asynchronous Start returning a Job for IDE/LSP integration.
// RunWithIndex and StartWithIndex accept a pre-parsed Navigator OpenAPI index for direct Go integration.
package barometer

import (
	"context"
	_ "embed"
	"errors"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/sailpoint-oss/barometer/internal/contract"
	"github.com/sailpoint-oss/barometer/internal/openapi"
	"github.com/sailpoint-oss/barometer/internal/reporter"
	"github.com/sailpoint-oss/barometer/internal/runner"
	navigator "github.com/sailpoint-oss/navigator"
)

//go:embed report-schema.json
var ReportSchemaJSON string

// Config is the contract test configuration (same as contract.Config).
type Config = contract.Config

// OpenAPIConfig configures an OpenAPI contract-test target.
type OpenAPIConfig = contract.OpenAPIConfig

// ArazzoConfig configures an Arazzo workflow target.
type ArazzoConfig = contract.ArazzoConfig

// Result is the contract test result.
type Result = contract.Result

// OpenAPIResult contains runtime results for OpenAPI contract tests.
type OpenAPIResult = contract.OpenAPIResult

// ArazzoResult contains runtime results for Arazzo workflow execution.
type ArazzoResult = contract.ArazzoResult

// WorkflowResult contains runtime results for an individual Arazzo workflow.
type WorkflowResult = contract.WorkflowResult

// OpenAPIContractResult is the per-operation runtime contract result shape.
type OpenAPIContractResult = openapi.ContractResult

// Client is the public runtime HTTP client type.
type Client = runner.Client

// ClientConfig configures the public runtime HTTP client.
type ClientConfig = runner.Config

// Format is the public contract report format type.
type Format = reporter.Format

// JSONReport is the versioned machine-readable report shape.
type JSONReport = reporter.JSONReport

// OpenAPIReport is the OpenAPI section of the machine-readable report.
type OpenAPIReport = reporter.OpenAPIReport

// ArazzoReport is the Arazzo section of the machine-readable report.
type ArazzoReport = reporter.ArazzoReport

const (
	FormatHuman       = reporter.FormatHuman
	FormatJUnit       = reporter.FormatJUnit
	FormatJSON        = reporter.FormatJSON
	JSONReportVersion = reporter.JSONReportVersion
)

var (
	// ErrConfigRequired indicates that no config was supplied to a run.
	ErrConfigRequired = contract.ErrConfigRequired
	// ErrTargetRequired indicates that no OpenAPI or Arazzo target was supplied.
	ErrTargetRequired = errors.New("at least one OpenAPI spec or Arazzo document is required")
)

// ContractInput is the flat integration input shape used by editor and
// orchestration callers that don't want to hand-build nested config structs.
type ContractInput struct {
	BaseURL         string
	Output          Format
	OpenAPISpec     string
	OpenAPITags     []string
	ArazzoDoc       string
	ArazzoWorkflows []string
}

// RunOpts configures RunWithIndex / StartWithIndex (optional client and operation filters).
type RunOpts struct {
	Client      *Client
	Tags        []string
	OperationID string
	// Credentials maps OpenAPI security scheme names to secret values (API keys, bearer tokens, etc.).
	Credentials map[string]string
}

// LoadConfig reads a config file from disk.
func LoadConfig(path string) (*Config, error) {
	return contract.LoadConfig(path)
}

// NewClient returns a public runtime HTTP client.
func NewClient(cfg *ClientConfig) (*Client, error) {
	return runner.NewClient(cfg)
}

// ToConfig converts flat integration inputs into the canonical nested config shape.
func (in ContractInput) ToConfig() (*Config, error) {
	spec := strings.TrimSpace(in.OpenAPISpec)
	doc := strings.TrimSpace(in.ArazzoDoc)
	if spec == "" && doc == "" {
		return nil, ErrTargetRequired
	}
	cfg := &Config{
		BaseURL: strings.TrimSpace(in.BaseURL),
	}
	if in.Output != "" {
		cfg.Output = string(in.Output)
	}
	if spec != "" {
		cfg.OpenAPI = &OpenAPIConfig{
			Spec: spec,
			Tags: append([]string(nil), in.OpenAPITags...),
		}
	}
	if doc != "" {
		cfg.Arazzo = &ArazzoConfig{
			Doc:       doc,
			Workflows: append([]string(nil), in.ArazzoWorkflows...),
		}
	}
	return cfg, nil
}

// RunInput converts flat integration inputs into config and executes the run.
func RunInput(ctx context.Context, input ContractInput, client *Client) (*Result, error) {
	cfg, err := input.ToConfig()
	if err != nil {
		return nil, err
	}
	return Run(ctx, cfg, client)
}

// StartInput converts flat integration inputs into config and starts an async run.
func StartInput(ctx context.Context, input ContractInput, client *Client) (*Job, error) {
	cfg, err := input.ToConfig()
	if err != nil {
		return nil, err
	}
	return Start(ctx, cfg, client)
}

// BuildReport returns the normalized JSON report model for a completed run.
func BuildReport(result *Result, duration time.Duration) JSONReport {
	return reporter.BuildContractReport(result, duration)
}

// WriteReport writes a normalized contract report in the requested format.
func WriteReport(w io.Writer, result *Result, format Format, duration time.Duration) error {
	return reporter.WriteContractReport(w, result, format, duration)
}

// WriteOpenAPIReport writes OpenAPI-only runtime results in the requested format.
func WriteOpenAPIReport(w io.Writer, results []OpenAPIContractResult, format Format, duration time.Duration) error {
	return reporter.WriteOpenAPIResults(w, results, format, duration)
}

// Run runs contract tests synchronously. It blocks until complete or context is cancelled.
func Run(ctx context.Context, cfg *Config, client *Client) (*Result, error) {
	if client == nil {
		var err error
		client, err = NewClient(nil)
		if err != nil {
			return nil, err
		}
	}
	return contract.Run(ctx, cfg, client)
}

// RunWithIndex runs contract tests against a pre-parsed OpenAPI index.
// This is the primary integration point for editor tooling and similar Go tools.
func RunWithIndex(ctx context.Context, idx *navigator.Index, baseURL string, opts *RunOpts) (*Result, error) {
	if idx == nil {
		return nil, nil
	}
	if opts == nil {
		opts = &RunOpts{}
	}
	client := opts.Client
	if client == nil {
		var err error
		client, err = NewClient(nil)
		if err != nil {
			return nil, err
		}
	}
	contractOpts := &openapi.ContractOpts{
		Tags: opts.Tags, OperationID: opts.OperationID,
		Credentials: opts.Credentials,
	}
	results, err := openapi.RunContract(ctx, idx, baseURL, client, contractOpts)
	if err != nil {
		return nil, err
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	return &Result{
		Pass: passed == len(results),
		OpenAPI: &contract.OpenAPIResult{
			Results: results,
			Passed:  passed,
			Total:   len(results),
		},
	}, nil
}

// StartWithIndex is the async version of RunWithIndex, returning a Job handle.
func StartWithIndex(ctx context.Context, idx *navigator.Index, baseURL string, opts *RunOpts) *Job {
	if idx == nil {
		return nil
	}
	if opts == nil {
		opts = &RunOpts{}
	}
	client := opts.Client
	if client == nil {
		var err error
		client, err = NewClient(nil)
		if err != nil {
			job := &Job{
				state: "failed",
				err:   err,
				done:  make(chan struct{}),
			}
			close(job.done)
			return job
		}
		opts.Client = client
	}
	ctx, cancel := context.WithCancel(ctx)
	job := &Job{
		state:  "pending",
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		defer close(job.done)
		job.mu.Lock()
		job.state = "running"
		job.mu.Unlock()
		result, err := RunWithIndex(ctx, idx, baseURL, opts)
		job.mu.Lock()
		defer job.mu.Unlock()
		if ctx.Err() != nil {
			job.state = "cancelled"
			job.err = ctx.Err()
		} else if err != nil {
			job.state = "failed"
			job.err = err
		} else {
			job.state = "completed"
			job.result = result
		}
	}()
	return job
}

// Job represents an asynchronous contract test run.
type Job struct {
	mu     sync.Mutex
	state  string // "pending", "running", "completed", "failed", "cancelled"
	result *Result
	err    error
	done   chan struct{}
	cancel context.CancelFunc
}

// Status returns the current job state and optional progress.
func (j *Job) Status() (state string, result *Result, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.state, j.result, j.err
}

// Wait blocks until the job completes, is cancelled, or the timeout elapses.
func (j *Job) Wait(timeout time.Duration) (*Result, error) {
	if timeout <= 0 {
		<-j.done
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.result, j.err
	}
	select {
	case <-j.done:
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.result, j.err
	case <-time.After(timeout):
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.result, context.DeadlineExceeded
	}
}

// Done returns a channel that is closed when the job completes.
func (j *Job) Done() <-chan struct{} {
	return j.done
}

// Cancel cancels the job's context. The job may not stop immediately.
func (j *Job) Cancel() {
	j.cancel()
}

// Start starts contract tests in the background. Returns immediately with a Job handle.
func Start(ctx context.Context, cfg *Config, client *Client) (*Job, error) {
	if cfg == nil {
		return nil, contract.ErrConfigRequired
	}
	if client == nil {
		var err error
		client, err = NewClient(nil)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	job := &Job{
		state:  "pending",
		done:   make(chan struct{}),
		cancel: cancel,
	}
	go func() {
		defer close(job.done)
		job.mu.Lock()
		job.state = "running"
		job.mu.Unlock()
		result, err := contract.Run(ctx, cfg, client)
		job.mu.Lock()
		defer job.mu.Unlock()
		if ctx.Err() != nil {
			job.state = "cancelled"
			job.err = ctx.Err()
		} else if err != nil {
			job.state = "failed"
			job.err = err
		} else {
			job.state = "completed"
			job.result = result
		}
	}()
	return job, nil
}
