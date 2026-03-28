// Package barometer exposes the stable module-root Go API for Barometer.
package barometer

import (
	"context"
	"io"
	"time"

	pkgbarometer "github.com/sailpoint-oss/barometer/pkg/barometer"
	navigator "github.com/sailpoint-oss/navigator"
)

var ReportSchemaJSON = pkgbarometer.ReportSchemaJSON

type Config = pkgbarometer.Config
type OpenAPIConfig = pkgbarometer.OpenAPIConfig
type ArazzoConfig = pkgbarometer.ArazzoConfig
type Result = pkgbarometer.Result
type OpenAPIResult = pkgbarometer.OpenAPIResult
type ArazzoResult = pkgbarometer.ArazzoResult
type WorkflowResult = pkgbarometer.WorkflowResult
type OpenAPIContractResult = pkgbarometer.OpenAPIContractResult
type Client = pkgbarometer.Client
type ClientConfig = pkgbarometer.ClientConfig
type Format = pkgbarometer.Format
type JSONReport = pkgbarometer.JSONReport
type OpenAPIReport = pkgbarometer.OpenAPIReport
type ArazzoReport = pkgbarometer.ArazzoReport
type ContractInput = pkgbarometer.ContractInput
type RunOpts = pkgbarometer.RunOpts
type Job = pkgbarometer.Job

const (
	FormatHuman       = pkgbarometer.FormatHuman
	FormatJUnit       = pkgbarometer.FormatJUnit
	FormatJSON        = pkgbarometer.FormatJSON
	JSONReportVersion = pkgbarometer.JSONReportVersion
)

var (
	ErrConfigRequired = pkgbarometer.ErrConfigRequired
	ErrTargetRequired = pkgbarometer.ErrTargetRequired
)

func LoadConfig(path string) (*Config, error) {
	return pkgbarometer.LoadConfig(path)
}

func NewClient(cfg *ClientConfig) *Client {
	return pkgbarometer.NewClient(cfg)
}

func Run(ctx context.Context, cfg *Config, client *Client) (*Result, error) {
	return pkgbarometer.Run(ctx, cfg, client)
}

func RunInput(ctx context.Context, input ContractInput, client *Client) (*Result, error) {
	return pkgbarometer.RunInput(ctx, input, client)
}

func RunWithIndex(ctx context.Context, idx *navigator.Index, baseURL string, opts *RunOpts) (*Result, error) {
	return pkgbarometer.RunWithIndex(ctx, idx, baseURL, opts)
}

func Start(ctx context.Context, cfg *Config, client *Client) (*Job, error) {
	return pkgbarometer.Start(ctx, cfg, client)
}

func StartInput(ctx context.Context, input ContractInput, client *Client) (*Job, error) {
	return pkgbarometer.StartInput(ctx, input, client)
}

func StartWithIndex(ctx context.Context, idx *navigator.Index, baseURL string, opts *RunOpts) *Job {
	return pkgbarometer.StartWithIndex(ctx, idx, baseURL, opts)
}

func BuildReport(result *Result, duration time.Duration) JSONReport {
	return pkgbarometer.BuildReport(result, duration)
}

func WriteReport(w io.Writer, result *Result, format Format, duration time.Duration) error {
	return pkgbarometer.WriteReport(w, result, format, duration)
}

func WriteOpenAPIReport(w io.Writer, results []OpenAPIContractResult, format Format, duration time.Duration) error {
	return pkgbarometer.WriteOpenAPIReport(w, results, format, duration)
}
