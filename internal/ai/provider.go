package ai

import (
	"context"
	"network-monitor/internal/diagnostic"
)

type Provider interface {
	Analyze(ctx context.Context, result diagnostic.DiagnosticResult) (string, error)
}
