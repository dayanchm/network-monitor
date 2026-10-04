package diagnostic

type DiagnosticResult struct {
	GatewayReachable bool   `json:"gateway_reachable"`
	GatewayMeasured  bool   `json:"gateway_measured"`
	GatewayStatus    string `json:"gateway_status"`
	GatewayTunnel    string `json:"gateway_tunnel,omitempty"`

	InternetReachable bool   `json:"internet_reachable"`
	InternetStatus    string `json:"internet_status"`
	InternetError     string `json:"internet_error,omitempty"`

	DNSHealthy bool   `json:"dns_healthy"`
	DNSError   string `json:"dns_error,omitempty"`

	LatencyMS          float64 `json:"latency_ms"`
	LatencyMeasured    bool    `json:"latency_measured"`
	LatencyLevel       string  `json:"latency_level,omitempty"`
	PacketLoss         float64 `json:"packet_loss"`
	PacketLossMeasured bool    `json:"packet_loss_measured"`

	VPNConnected bool `json:"vpn_connected"`
	VPNKnown     bool `json:"vpn_known"`
}
