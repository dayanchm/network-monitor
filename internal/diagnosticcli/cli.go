package diagnosticcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"network-monitor/internal/ai"
	"network-monitor/internal/diagnostic"
	"os"
	"strings"
)

// RunDiagnose parses CLI arguments and prints the network diagnostic summary.
func RunDiagnose(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	options, err := parseOptions(args, stderr)
	host := options.host
	if err != nil {
		return err
	}
	var result diagnostic.DiagnosticResult
	if host == "" {
		result = diagnostic.Diagnose(ctx)
	} else {
		result = diagnostic.DiagnoseHost(ctx, host)
	}
	if err := result.Print(stdout); err != nil {
		return err
	}
	if !options.ai {
		return nil
	}
	provider := &ai.OllamaProvider{Model: options.model, BaseURL: options.ollamaURL}
	return printAnalysis(ctx, provider, result, stdout)
}

type diagnoseOptions struct {
	host             string
	ai               bool
	model, ollamaURL string
}

func parseDiagnoseArgs(args []string, output io.Writer) (string, error) {
	options, err := parseOptions(args, output)
	return options.host, err
}

func parseOptions(args []string, output io.Writer) (diagnoseOptions, error) {
	options := diagnoseOptions{}

	if len(args) == 0 || args[0] != "diagnose" {
		return options, fmt.Errorf("Usage: network-monitor [diagnose [--host hostname]]")
	}
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.BoolVar(&options.ai, "ai", false, "analyze collected measurements with local Ollama")
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = ai.DefaultModel
	}
	endpoint := os.Getenv("OLLAMA_HOST")
	if endpoint == "" {
		endpoint = ai.DefaultBaseURL
	}
	flags.StringVar(&options.model, "model", model, "Ollama model name")
	flags.StringVar(&options.ollamaURL, "ollama-url", endpoint, "Ollama server URL")
	host := flags.String("host", "", "IPv4 address or hostname to diagnose (no URL or port)")
	if err := flags.Parse(args[1:]); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, fmt.Errorf("unexpected argument: %s", flags.Arg(0))
	}
	supplied := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "host" {
			supplied = true
		}
	})
	if supplied {
		if *host == "" || strings.HasPrefix(*host, "-") || strings.ContainsAny(*host, "/: \t\r\n") {
			return options, fmt.Errorf("--host requires a hostname or IPv4 address, without a URL or port")
		}
	}
	options.host = *host
	return options, nil
}

func printAnalysis(ctx context.Context, provider ai.Provider, result diagnostic.DiagnosticResult, output io.Writer) error {
	if _, err := fmt.Fprint(output, "\nAI Analysis\n\n"); err != nil {
		return err
	}
	analysis, err := provider.Analyze(ctx, result)
	if err != nil {
		_, writeErr := fmt.Fprintf(output, "Unavailable: %v\n", err)
		return writeErr
	}
	_, err = fmt.Fprintln(output, analysis)
	return err
}
