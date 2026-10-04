package ai

import (
	"context"
	"network-monitor/internal/diagnostic"
)

type MockProvider struct {
	Response string
	Err      error
}

func (m MockProvider) Analyze(
	ctx context.Context,
	result diagnostic.DiagnosticResult,
) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}
