package diagnostic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

type diagnosticChecks struct {
	target         string
	gateway        func(context.Context) (string, error)
	ping           func(context.Context, string) (string, error)
	dns            func(context.Context) error
	routeInterface func(context.Context) (string, error)
}

func Diagnose(ctx context.Context) DiagnosticResult {
	return diagnoseTarget(ctx, "8.8.8.8", "google.com")
}

// DiagnoseHost uses a custom IPv4 host for DNS, ping, and route checks.
func DiagnoseHost(ctx context.Context, host string) DiagnosticResult {
	return diagnoseTarget(ctx, host, host)
}

func diagnoseTarget(ctx context.Context, target, dnsHost string) DiagnosticResult {
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

func runDiagnostics(ctx context.Context, checks diagnosticChecks) DiagnosticResult {
	result := DiagnosticResult{GatewayStatus: "unavailable", InternetStatus: "unavailable"}

	// Give each check its own deadline so a failed gateway does not skip later checks.
	gatewayCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	gateway, err := checks.gateway(gatewayCtx)
	cancel()
	if err == nil {
		pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		output, pingErr := checks.ping(pingCtx, gateway)
		cancel()
		result.GatewayStatus = pingStatus(output, pingErr)
		result.GatewayReachable = result.GatewayStatus == "reachable"
	} else {
		var tunnel *tunnelGatewayError
		if errors.As(err, &tunnel) {
			result.GatewayTunnel = tunnel.iface
		}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	target := checks.target
	if target == "" {
		target = "8.8.8.8"
	}
	output, pingErr := checks.ping(pingCtx, target)
	cancel()
	result.InternetStatus = pingStatus(output, pingErr)
	result.InternetReachable = result.InternetStatus == "reachable" // İnternet hedefinin erişim sonucu.
	if result.InternetStatus == "unavailable" && pingErr != nil {
		result.InternetError = pingErr.Error()
	}
	if loss, latency, ok := parsePing(output); ok {
		result.PacketLoss = loss // Yüzde değerini metin yerine sayı olarak saklar.
		result.PacketLossMeasured = true
		if latency != "" {
			if ms, err := strconv.ParseFloat(latency, 64); err == nil {
				result.LatencyMS = ms // Ortalama gecikmeyi milisaniye olarak saklar.
				result.LatencyMeasured = true
			}
		}
	}
	dnsCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	if err := checks.dns(dnsCtx); err == nil {
		result.DNSHealthy = true
	} else {
		result.DNSError = err.Error()
	}
	cancel()
	routeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	if iface, err := checks.routeInterface(routeCtx); err == nil {
		result.VPNKnown = true
		result.VPNConnected = isTunnelInterface(iface)
	}
	cancel()

	return result
}
