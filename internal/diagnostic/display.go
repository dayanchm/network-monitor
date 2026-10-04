package diagnostic

import (
	"fmt"
	"io"
)

// Diagnostics is used only for terminal formatting.
type Diagnostics struct{ Gateway, Internet, DNS, Latency, PacketLoss, VPN string }

func (r DiagnosticResult) summary() Diagnostics {
	d := Diagnostics{Gateway: r.GatewayStatus, Internet: r.InternetStatus, DNS: "healthy", Latency: "unavailable", PacketLoss: "unavailable", VPN: "unknown"}
	if d.Gateway == "" {
		d.Gateway = "unavailable"
	}
	if d.Internet == "" {
		d.Internet = "unavailable"
	}
	if r.GatewayTunnel != "" {
		d.Gateway = fmt.Sprintf("tunnel route (%s)", r.GatewayTunnel)
	}
	if r.InternetError != "" && d.Internet == "unavailable" {
		d.Internet += " (" + r.InternetError + ")"
	}
	if !r.DNSHealthy {
		d.DNS = "unhealthy"
		if r.DNSError != "" {
			d.DNS += " (" + r.DNSError + ")"
		}
	}
	if r.LatencyMeasured {
		d.Latency = fmt.Sprintf("%gms", r.LatencyMS)
	}
	if r.PacketLossMeasured {
		d.PacketLoss = fmt.Sprintf("%g%%", r.PacketLoss)
	}
	if r.VPNKnown {
		d.VPN = "disconnected"
		if r.VPNConnected {
			d.VPN = "connected"
		}
	}
	return d
}

func (r DiagnosticResult) Print(w io.Writer) error {
	d := r.summary()
	_, err := fmt.Fprintf(w, "Network Diagnostics\n\nGateway:     %s\nInternet:    %s\nDNS:         %s\nLatency:     %s\nPacket loss: %s\nVPN:         %s\n", d.Gateway, d.Internet, d.DNS, d.Latency, d.PacketLoss, d.VPN)
	return err
}
