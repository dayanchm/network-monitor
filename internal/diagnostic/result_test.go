package diagnostic

import (
	"context"
	"encoding/json"
	"testing"
)

func TestStructuredResult(t *testing.T) {
	checks := diagnosticChecks{
		gateway: func(context.Context) (string, error) { return "192.0.2.1", nil },
		ping: func(context.Context, string) (string, error) {
			return "4 packets transmitted, 3 received, 25% packet loss\nrtt min/avg/max/mdev = 40/42.5/45/1 ms", nil
		},
		dns:            func(context.Context) error { return nil },
		routeInterface: func(context.Context) (string, error) { return "utun4", nil },
	}
	got := runDiagnostics(context.Background(), checks)
	if !got.GatewayReachable || !got.InternetReachable || !got.DNSHealthy || !got.VPNConnected || got.LatencyMS != 42.5 || got.PacketLoss != 25 {
		t.Fatalf("%+v", got)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gateway_reachable", "internet_reachable", "dns_healthy", "latency_ms", "packet_loss", "vpn_connected"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("missing JSON key %s", key)
		}
	}
	var decoded DiagnosticResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != got {
		t.Fatalf("round-trip changed result: %+v", decoded)
	}
}
