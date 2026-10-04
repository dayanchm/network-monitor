package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Diagnostics contains human-readable results, including unavailable checks.
type Diagnostics struct {
	Gateway, Internet, DNS, Latency, PacketLoss, VPN string
}

func (d Diagnostics) Print(w io.Writer) error {
	_, err := fmt.Fprintf(w, "Network Diagnostics\n\nGateway:     %s\nInternet:    %s\nDNS:         %s\nLatency:     %s\nPacket loss: %s\nVPN:         %s\n", d.Gateway, d.Internet, d.DNS, d.Latency, d.PacketLoss, d.VPN)
	return err
}

type diagnosticChecks struct {
	target         string
	gateway        func(context.Context) (string, error)
	ping           func(context.Context, string) (string, error)
	dns            func(context.Context) error
	routeInterface func(context.Context) (string, error)
}

// Diagnose runs bounded checks using the default IPv4 route and public targets.
func Diagnose(ctx context.Context) Diagnostics { return diagnoseTarget(ctx, "8.8.8.8", "google.com") }

// DiagnoseHost uses a custom IPv4 host for DNS, ping, and route checks.
func DiagnoseHost(ctx context.Context, host string) Diagnostics {
	return diagnoseTarget(ctx, host, host)
}

func diagnoseTarget(ctx context.Context, target, dnsHost string) Diagnostics {
	return runDiagnostics(ctx, diagnosticChecks{
		target:  target,
		gateway: defaultGateway,
		ping: func(ctx context.Context, host string) (string, error) {
			ip, err := resolveDiagnosticIPv4(ctx, host)
			if err != nil {
				return "", err
			}
			return diagnosticPing(ctx, ip)
		},
		dns: func(ctx context.Context) error {
			addresses, err := net.DefaultResolver.LookupHost(ctx, dnsHost)
			if err == nil && len(addresses) == 0 {
				return fmt.Errorf("no addresses returned")
			}
			return err
		},
		routeInterface: func(ctx context.Context) (string, error) {
			ip, err := resolveDiagnosticIPv4(ctx, target)
			if err != nil {
				return "", err
			}
			return internetRouteInterface(ctx, ip)
		},
	})
}

func runDiagnostics(ctx context.Context, checks diagnosticChecks) Diagnostics {
	d := Diagnostics{Gateway: "unavailable", Internet: "unavailable", DNS: "unhealthy", Latency: "unavailable", PacketLoss: "unavailable", VPN: "unknown"}
	// Give each check its own deadline so a failed gateway does not skip later checks.
	gatewayCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	gateway, err := checks.gateway(gatewayCtx)
	cancel()
	if err == nil {
		pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		output, pingErr := checks.ping(pingCtx, gateway)
		cancel()
		d.Gateway = pingStatus(output, pingErr)
	} else {
		var tunnel *tunnelGatewayError
		if errors.As(err, &tunnel) {
			d.Gateway = fmt.Sprintf("tunnel route (%s)", tunnel.iface)
		}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	target := checks.target
	if target == "" {
		target = "8.8.8.8"
	}
	output, pingErr := checks.ping(pingCtx, target)
	cancel()
	d.Internet = pingStatus(output, pingErr)
	if d.Internet == "unavailable" && pingErr != nil {
		d.Internet += " (" + pingErr.Error() + ")"
	}
	if loss, latency, ok := parsePing(output); ok {
		d.PacketLoss = fmt.Sprintf("%g%%", loss)
		if latency != "" {
			d.Latency = latency + "ms"
		}
	}
	dnsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	if err := checks.dns(dnsCtx); err == nil {
		d.DNS = "healthy"
	} else {
		d.DNS = "unhealthy (" + err.Error() + ")"
	}
	cancel()
	routeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	if iface, err := checks.routeInterface(routeCtx); err == nil {
		d.VPN = "disconnected"
		if isTunnelInterface(iface) {
			d.VPN = "connected"
		}
	}
	cancel()

	return d
}

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

// A tunnel route can carry traffic without an IP next hop to ping.
type tunnelGatewayError struct{ iface string }

func (e *tunnelGatewayError) Error() string { return "default route uses tunnel " + e.iface }

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

func resolveDiagnosticIPv4(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return "", fmt.Errorf("IPv4 target required")
		}
		return ip.String(), nil
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return "", err
	}
	if len(addresses) == 0 {
		return "", fmt.Errorf("no IPv4 address for %s", host)
	}
	return addresses[0].String(), nil
}
