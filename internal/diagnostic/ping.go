package diagnostic

import (
	"context"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
)

var lossPattern = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)% packet loss`)
var latencyPattern = regexp.MustCompile(`(?:rtt|round-trip)[^=]*=\s*[0-9.]+/([0-9.]+)/`)

func parsePing(output string) (float64, string, bool) {
	match := lossPattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, "", false
	}
	loss, err := strconv.ParseFloat(match[1], 64)
	if err != nil || loss < 0 || loss > 100 {
		return 0, "", false
	}
	latency := ""
	if match := latencyPattern.FindStringSubmatch(output); len(match) == 2 {
		latency = match[1]
	}
	return loss, latency, true
}

func pingStatus(output string, err error) string {
	if loss, _, ok := parsePing(output); ok {
		if loss < 100 {
			return "reachable"
		}
		return "unreachable"
	}
	if err == nil {
		return "reachable"
	}
	return "unavailable"
}

func diagnosticPing(ctx context.Context, host string) (string, error) {
	args := []string{"-n", "-c", "4"}
	// macOS uses milliseconds for -W; Linux uses seconds. Bound reply waits
	// so total loss still produces ping's summary before our outer deadline.
	switch runtime.GOOS {
	case "darwin":
		args = append(args, "-W", "1000")
	case "linux":
		args = append(args, "-W", "1")
	}
	args = append(args, host)
	output, err := exec.CommandContext(ctx, "ping", args...).CombinedOutput()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	return string(output), err
}
