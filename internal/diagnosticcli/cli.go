package diagnosticcli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"network-monitor/internal/diagnostic"
	"strings"
)

// RunDiagnose parses CLI arguments and prints the network diagnostic summary.
func RunDiagnose(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	host, err := parseDiagnoseArgs(args, stderr)
	if err != nil {
		return err
	}
	var result diagnostic.DiagnosticResult
	if host == "" {
		result = diagnostic.Diagnose(ctx)
	} else {
		result = diagnostic.DiagnoseHost(ctx, host)
	}
	return result.Print(stdout)
}

func parseDiagnoseArgs(args []string, output io.Writer) (string, error) {
	if len(args) == 0 || args[0] != "diagnose" {
		return "", fmt.Errorf("Usage: network-monitor [diagnose [--host hostname]]")
	}
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(output)
	host := flags.String("host", "", "IPv4 address or hostname to diagnose (no URL or port)")
	if err := flags.Parse(args[1:]); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected argument: %s", flags.Arg(0))
	}
	supplied := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "host" {
			supplied = true
		}
	})
	if supplied {
		if *host == "" || strings.HasPrefix(*host, "-") || strings.ContainsAny(*host, "/: \t\r\n") {
			return "", fmt.Errorf("--host requires a hostname or IPv4 address, without a URL or port")
		}
	}
	return *host, nil
}
