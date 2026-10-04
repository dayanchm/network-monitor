package diagnostic

type DiagnosticResult struct {
	GatewayReachable   bool    `json:"gateway_reachable"`
	InternetReachable  bool    `json:"internet_reachable"`
	DNSHealthy         bool    `json:"dns_healthy"`
	LatencyMS          float64 `json:"latency_ms"`
	PacketLoss         float64 `json:"packet_loss"`
	VPNConnected       bool    `json:"vpn_connected"`
	GatewayStatus      string  `json:"gateway_status"`
	InternetStatus     string  `json:"internet_status"`
	GatewayTunnel      string  `json:"gateway_tunnel,omitempty"`
	InternetError      string  `json:"internet_error,omitempty"`
	DNSError           string  `json:"dns_error,omitempty"`
	LatencyMeasured    bool    `json:"latency_measured"`
	PacketLossMeasured bool    `json:"packet_loss_measured"`
	VPNKnown           bool    `json:"vpn_known"`
}
