package network

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParsePing(t *testing.T) {
	for _, tc := range []struct {
		name, output, latency string
		loss                  float64
		ok                    bool
	}{
		{"linux", "4 packets transmitted, 4 received, 0% packet loss\nrtt min/avg/max/mdev = 40.0/42.0/44.0/1.0 ms", "42.0", 0, true},
		{"macOS partial", "4 packets transmitted, 3 packets received, 25.0% packet loss\nround-trip min/avg/max/stddev = 1.0/2.5/4.0/1.0 ms", "2.5", 25, true},
		{"total loss", "4 packets transmitted, 0 received, 100% packet loss", "", 100, true},
		{"missing tool", "ping: command not found", "", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loss, latency, ok := parsePing(tc.output)
			if loss != tc.loss || latency != tc.latency || ok != tc.ok {
				t.Fatalf("got %v %q %v", loss, latency, ok)
			}
		})
	}
}

func TestParseGateway(t *testing.T) {
	for _, output := range []string{"gateway: 192.168.1.1\n", "default via 192.168.1.1 dev eth0 proto dhcp\n"} {
		got, err := parseGateway(output)
		if got != "192.168.1.1" || err != nil {
			t.Fatalf("got %q, %v", got, err)
		}
	}
	for _, output := range []string{"", "default dev tun0", "gateway: invalid"} {
		if _, err := parseGateway(output); err == nil {
			t.Fatalf("accepted %q", output)
		}
	}
}

func TestDiagnostics(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "healthy", true: "failed"}[failed], func(t *testing.T) {
			calls := 0
			checks := diagnosticChecks{
				gateway: func(ctx context.Context) (string, error) {
					if _, ok := ctx.Deadline(); !ok {
						t.Error("missing deadline")
					}
					if failed {
						return "", errors.New("no route")
					}
					return "192.168.1.1", nil
				},
				ping: func(ctx context.Context, host string) (string, error) {
					calls++
					if _, ok := ctx.Deadline(); !ok {
						t.Error("missing deadline")
					}
					if failed {
						return "4 packets transmitted, 0 received, 100% packet loss", errors.New("exit 1")
					}
					return "4 packets transmitted, 4 received, 0% packet loss\nrtt min/avg/max/mdev = 40/42/44/1 ms", nil
				},
				dns: func(context.Context) error {
					if failed {
						return errors.New("lookup failed")
					}
					return nil
				},
				routeInterface: func(context.Context) (string, error) { return "utun0", nil },
			}
			got := runDiagnostics(context.Background(), checks)
			want := Diagnostics{"reachable", "reachable", "healthy", "42ms", "0%", "connected"}
			expectedCalls := 2
			if failed {
				want = Diagnostics{"unavailable", "unreachable", "unhealthy (lookup failed)", "unavailable", "100%", "connected"}
				expectedCalls = 1
			}
			if got != want || calls != expectedCalls {
				t.Fatalf("got %+v (%d calls), want %+v", got, calls, want)
			}
			var buffer bytes.Buffer
			if err := got.Print(&buffer); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(buffer.String(), "Network Diagnostics\n\nGateway:     ") || !strings.Contains(buffer.String(), "Packet loss: "+want.PacketLoss) {
				t.Fatal(buffer.String())
			}
		})
	}
}

func TestUnavailableChecks(t *testing.T) {
	checks := diagnosticChecks{
		gateway:        func(context.Context) (string, error) { return "", errors.New("missing route") },
		ping:           func(context.Context, string) (string, error) { return "", errors.New("missing ping") },
		dns:            func(context.Context) error { return errors.New("DNS failed") },
		routeInterface: func(context.Context) (string, error) { return "", errors.New("route failure") },
	}
	got := runDiagnostics(context.Background(), checks)
	if got.Internet != "unavailable (missing ping)" || got.PacketLoss != "unavailable" || got.VPN != "unknown" {
		t.Fatalf("%+v", got)
	}
	checks.routeInterface = func(context.Context) (string, error) { return "en0", nil }

	if got := runDiagnostics(context.Background(), checks); got.VPN != "disconnected" {
		t.Fatalf("%+v", got)
	}
}

func TestRouteVPNDetection(t *testing.T) {
	for _, tc := range []struct {
		output, iface string
		tunnel        bool
	}{
		{"route to: 8.8.8.8\n interface: en0\n", "en0", false},
		{"route to: 8.8.8.8\n interface: utun4\n", "utun4", true},
		{"8.8.8.8 via 192.168.1.1 dev eth0 src 192.168.1.2", "eth0", false},
		{"8.8.8.8 dev wg0 table 51820", "wg0", true},
	} {
		iface, err := parseRouteInterface(tc.output)
		if err != nil || iface != tc.iface || isTunnelInterface(iface) != tc.tunnel {
			t.Fatalf("%q: %q, %v", tc.output, iface, err)
		}
	}
	if _, err := parseRouteInterface("route unavailable"); err == nil {
		t.Fatal("expected error")
	}
}

func TestTunnelGateway(t *testing.T) {
	for _, output := range []string{"route to: default\ninterface: utun4\n", "default dev tun0"} {
		_, err := parseGateway(output)
		var tunnel *tunnelGatewayError
		if !errors.As(err, &tunnel) {
			t.Fatalf("expected tunnel route, got %v", err)
		}
		calls := 0
		checks := diagnosticChecks{
			gateway: func(context.Context) (string, error) { return "", err },
			ping: func(context.Context, string) (string, error) {
				calls++
				return "4 packets transmitted, 4 received, 0% packet loss", nil
			},
			dns:            func(context.Context) error { return nil },
			routeInterface: func(context.Context) (string, error) { return tunnel.iface, nil },
		}
		got := runDiagnostics(context.Background(), checks)
		if got.Gateway != "tunnel route ("+tunnel.iface+")" || got.Internet != "reachable" || calls != 1 {
			t.Fatalf("%+v, ping calls %d", got, calls)
		}
	}
}

func TestCustomTarget(t *testing.T) {
	checks := diagnosticChecks{
		target:  "tmcars.info",
		gateway: func(context.Context) (string, error) { return "", errors.New("no route") },
		ping: func(_ context.Context, host string) (string, error) {
			if host != "tmcars.info" {
				t.Fatalf("wrong target: %s", host)
			}
			return "4 packets transmitted, 4 received, 0% packet loss", nil
		},
		dns:            func(context.Context) error { return nil },
		routeInterface: func(context.Context) (string, error) { return "en0", nil },
	}
	if got := runDiagnostics(context.Background(), checks); got.Internet != "reachable" {
		t.Fatalf("%+v", got)
	}
	ip, err := resolveDiagnosticIPv4(context.Background(), "192.0.2.1")
	if err != nil || ip != "192.0.2.1" {
		t.Fatalf("%s, %v", ip, err)
	}
}
