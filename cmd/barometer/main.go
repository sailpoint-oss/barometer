// Command barometer is the CLI for OpenAPI and Arazzo contract testing.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	barometerapi "github.com/sailpoint-oss/barometer"
	"github.com/sailpoint-oss/barometer/internal/arazzo"
	"github.com/sailpoint-oss/barometer/internal/openapi"
	"github.com/spf13/cobra"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "barometer",
	Short: "OpenAPI and Arazzo contract testing",
	Long:  "Barometer runs contract tests and Arazzo workflows against APIs using OpenAPI and Arazzo documents.",
}

var openapiValidateCmd = &cobra.Command{
	Use:   "validate [spec]",
	Short: "Validate an OpenAPI 3.x document (syntax and structure)",
	Args:  cobra.ExactArgs(1),
	RunE:  runOpenAPIValidate,
}

var openapiTestTags []string
var openapiTestOutput string
var openapiTestProxyURL string

var openapiTestCmd = &cobra.Command{
	Use:   "test [spec] [base-url]",
	Short: "Run contract tests from OpenAPI spec against base URL",
	Args:  cobra.ExactArgs(2),
	RunE:  runOpenAPITest,
}

var openapiCmd = &cobra.Command{
	Use:   "openapi",
	Short: "OpenAPI validation and contract testing",
}

var arazzoValidateCmd = &cobra.Command{
	Use:   "validate [doc]",
	Short: "Validate Arazzo document and referenced sources",
	Args:  cobra.ExactArgs(1),
	RunE:  runArazzoValidate,
}

var arazzoRunCmd = &cobra.Command{
	Use:   "run [doc] [base-url]",
	Short: "Run Arazzo workflows against base URL",
	Args:  cobra.ExactArgs(2),
	RunE:  runArazzoRun,
}

var arazzoWorkflowID string
var arazzoRunProxyURL string

var arazzoCmd = &cobra.Command{
	Use:   "arazzo",
	Short: "Arazzo workflow validation and execution",
}

var contractTestConfig string
var contractTestOutput string
var contractTestProxyURL string

var contractTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Run contract tests from config file",
	RunE:  runContractTest,
}

var contractCmd = &cobra.Command{
	Use:   "contract",
	Short: "Unified contract testing (OpenAPI + Arazzo)",
}

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the JSON Schema for the contract report format",
	RunE:  runSchema,
}

func init() {
	rootCmd.AddCommand(openapiCmd)
	openapiCmd.AddCommand(openapiValidateCmd)
	openapiCmd.AddCommand(openapiTestCmd)
	openapiTestCmd.Flags().StringSliceVar(&openapiTestTags, "tags", nil, "Run only operations with these tags")
	openapiTestCmd.Flags().StringVarP(&openapiTestOutput, "output", "o", "human", "Output format: human, junit, json")
	openapiTestCmd.Flags().StringVar(&openapiTestProxyURL, "proxy-url", "", "Route runtime requests through this proxy URL")

	rootCmd.AddCommand(arazzoCmd)
	arazzoCmd.AddCommand(arazzoValidateCmd)
	arazzoCmd.AddCommand(arazzoRunCmd)
	arazzoRunCmd.Flags().StringVar(&arazzoWorkflowID, "workflow", "", "Run only this workflow ID")
	arazzoRunCmd.Flags().StringVar(&arazzoRunProxyURL, "proxy-url", "", "Route runtime requests through this proxy URL")

	rootCmd.AddCommand(contractCmd)
	contractCmd.AddCommand(contractTestCmd)
	contractTestCmd.Flags().StringVarP(&contractTestConfig, "config", "c", "barometer.yaml", "Config file path")
	contractTestCmd.Flags().StringVarP(&contractTestOutput, "output", "o", "human", "Output format: human, junit, json")
	contractTestCmd.Flags().StringVar(&contractTestProxyURL, "proxy-url", "", "Override proxy URL from config")

	rootCmd.AddCommand(schemaCmd)
}

func runSchema(cmd *cobra.Command, args []string) error {
	fmt.Print(barometerapi.ReportSchemaJSON)
	return nil
}

func runArazzoValidate(cmd *cobra.Command, args []string) error {
	doc, err := arazzo.Load(args[0])
	if err != nil {
		return err
	}
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Valid Arazzo %s document: %s\n", doc.Arazzo, args[0])
	return nil
}

func runContractTest(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	start := time.Now()
	cfg, err := barometerapi.LoadConfig(contractTestConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if strings.TrimSpace(contractTestProxyURL) != "" {
		cfg.ProxyURL = contractTestProxyURL
	}
	client, err := barometerapi.NewClient(&barometerapi.ClientConfig{ProxyURL: cfg.ProxyURL})
	if err != nil {
		return err
	}
	result, err := barometerapi.Run(ctx, cfg, client)
	if err != nil {
		return err
	}
	duration := time.Since(start)
	format := barometerapi.Format(contractTestOutput)
	if format != barometerapi.FormatHuman && format != barometerapi.FormatJUnit && format != barometerapi.FormatJSON {
		format = barometerapi.FormatHuman
	}
	out := os.Stdout
	if format == barometerapi.FormatHuman {
		out = os.Stderr
	}
	if err := barometerapi.WriteReport(out, result, format, duration); err != nil {
		return err
	}
	if !result.Pass {
		return fmt.Errorf("contract test failed")
	}
	return nil
}

func runArazzoRun(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	docPath := args[0]
	baseURL := args[1]
	doc, err := arazzo.Load(docPath)
	if err != nil {
		return err
	}
	if err := doc.Validate(); err != nil {
		return err
	}
	client, err := barometerapi.NewClient(&barometerapi.ClientConfig{ProxyURL: arazzoRunProxyURL})
	if err != nil {
		return err
	}
	workflowIDs := []string{arazzoWorkflowID}
	if arazzoWorkflowID == "" {
		for _, w := range doc.Workflows {
			workflowIDs = append(workflowIDs, w.WorkflowID)
		}
		workflowIDs = workflowIDs[1:]
	}
	inputs := make(map[string]any)
	for _, wfID := range workflowIDs {
		out, err := doc.RunWorkflow(ctx, wfID, baseURL, inputs, client)
		if err != nil {
			return fmt.Errorf("workflow %q: %w", wfID, err)
		}
		fmt.Fprintf(os.Stderr, "Workflow %q completed. Outputs: %v\n", wfID, out)
	}
	return nil
}

func runOpenAPIValidate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	specPath := args[0]
	idx, err := openapi.Load(ctx, specPath, nil)
	if err != nil {
		return err
	}
	if err := openapi.Validate(idx); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Valid OpenAPI %s document: %s\n", openapi.Version(idx), specPath)
	return nil
}

func runOpenAPITest(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	start := time.Now()
	specPath := args[0]
	baseURL := args[1]
	idx, err := openapi.Load(ctx, specPath, nil)
	if err != nil {
		return err
	}
	if err := openapi.Validate(idx); err != nil {
		return fmt.Errorf("spec validation failed: %w", err)
	}
	client, err := barometerapi.NewClient(&barometerapi.ClientConfig{ProxyURL: openapiTestProxyURL})
	if err != nil {
		return err
	}
	responseValidator, err := openapi.NewLibOpenAPIResponseValidator(ctx, specPath, client.Client)
	if err != nil {
		return err
	}
	opts := &openapi.ContractOpts{Tags: openapiTestTags, ResponseValidator: responseValidator}
	results, err := openapi.RunContract(ctx, idx, baseURL, client, opts)
	if err != nil {
		return err
	}
	duration := time.Since(start)
	format := barometerapi.Format(openapiTestOutput)
	if format != barometerapi.FormatHuman && format != barometerapi.FormatJUnit && format != barometerapi.FormatJSON {
		format = barometerapi.FormatHuman
	}
	out := os.Stdout
	if format == barometerapi.FormatHuman {
		out = os.Stderr
	}
	if err := barometerapi.WriteOpenAPIReport(out, results, format, duration); err != nil {
		return err
	}
	passed := 0
	for _, r := range results {
		if r.Pass {
			passed++
		}
	}
	if passed < len(results) {
		return fmt.Errorf("contract test failed: %d/%d passed", passed, len(results))
	}
	return nil
}
