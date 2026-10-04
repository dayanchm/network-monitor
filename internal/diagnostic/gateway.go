package diagnostic

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

func defaultGateway(ctx context.Context) (string, error) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "route", "-n", "get", "default")
	case "linux":
		command = exec.CommandContext(ctx, "ip", "-4", "route", "show", "default")
	default:
		return "", fmt.Errorf("gateway detection unsupported on %s", runtime.GOOS)
	}
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return parseGateway(string(output))
}

func parseGateway(output string) (string, error) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "gateway:" && net.ParseIP(fields[1]) != nil {
			return fields[1], nil
		}
		if len(fields) > 0 && fields[0] == "default" {
			for i := 1; i+1 < len(fields); i++ {
				if fields[i] == "via" && net.ParseIP(fields[i+1]) != nil {
					return fields[i+1], nil
				}
			}
		}
	}
	if iface, err := parseRouteInterface(output); err == nil && isTunnelInterface(iface) {
		return "", &tunnelGatewayError{iface: iface}
	}
	return "", fmt.Errorf("no default gateway found")
}

// A tunnel route can carry traffic without an IP next hop to ping.
type tunnelGatewayError struct{ iface string }

func (e *tunnelGatewayError) Error() string { return "default route uses tunnel " + e.iface }
