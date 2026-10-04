package diagnostic

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func internetRouteInterface(ctx context.Context, target string) (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "route", "-n", "get", target)
	case "linux":
		command = exec.CommandContext(ctx, "ip", "-4", "route", "get", target)
	default:
		return "", fmt.Errorf("route detection unsupported on %s", runtime.GOOS)
	}
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return parseRouteInterface(string(output))
}

func parseRouteInterface(output string) (string, error) {
	fields := strings.Fields(output)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == "interface:" || fields[i] == "dev" {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("no route interface found")
}

func isTunnelInterface(name string) bool {
	name = strings.ToLower(name)
	for _, prefix := range []string{"utun", "tun", "tap", "wg", "ppp", "ipsec"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
