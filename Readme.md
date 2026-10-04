# Network Monitor

A lightweight Go tool for listing devices on your home network, monitoring local traffic, and preparing device-based daily usage limits.

## Features

- Lists local network devices with IP, MAC, and hostname information.
- Automatically selects the active network interface for traffic capture.
- Lets you define usage limits per device in GB.
- Exposes a simple REST API for dashboards, scripts, or automation.

## Running at Home

1. Download dependencies:

```bash
go mod download
```

2. Optional, but recommended if you want to discover as many home-network devices as possible:

```bash
brew install arp-scan
```

Without `arp-scan`, the app falls back to the system ARP cache, which may only include devices your computer has recently seen.

3. Start the application:

```bash
sudo go run .
```

`sudo` is usually required on macOS/Linux for packet capture.

4. Check it from the same computer:

```bash
curl http://localhost:8888/api/devices
```

5. Open the web dashboard:

```text
http://localhost:8888
```

6. To access it from another device on your home network, use your computer's local IP address:

```bash
curl http://192.168.1.4:8888/api/devices
```

The active local address on this machine is currently `192.168.1.4`. If your home network changes, check the new IP address with `ifconfig` or your system network settings.

## Configuration

By default, the server listens on `0.0.0.0:8888`, which makes it reachable from other devices on the same local network.

```bash
HOST=0.0.0.0 PORT=8888 sudo -E go run .
```

If the network interface cannot be detected automatically, or if you want to choose it manually:

```bash
NETWORK_INTERFACE=en1 sudo -E go run .
```

On macOS, the Wi-Fi or Ethernet interface can vary by device, for example `en0` or `en1`. To inspect active interfaces:

```bash
ifconfig
```

## API

### List Devices

```bash
curl http://localhost:8888/api/devices
```

### Stream Live Traffic

```bash
curl http://localhost:8888/api/traffic
```

### Set a Device Limit

```bash
curl -X POST http://localhost:8888/api/limit \
  -H "Content-Type: application/json" \
  -d '{"mac":"aa:bb:cc:dd:ee:ff","limit_gb":1}'
```

## Notes

- This tool is not a router; it can only capture traffic visible to the computer running it.
- To actually block a device, you need to add router or firewall integration.
- Do not expose this service to the public internet. Use it only inside your trusted local network.

## Network diagnostics

Run all checks without starting the web server, packet capture, or DNS proxy:

```bash
go build -o network-monitor .
./network-monitor diagnose
# Or: go run . diagnose
```

Example output (results depend on your network):

```text
Network Diagnostics

Gateway:     reachable
Internet:    reachable
DNS:         healthy
Latency:     42ms
Packet loss: 0%
VPN:         connected
```

Diagnostics support macOS and Linux and use the system `ping` command. Gateway
lookup requires `route` on macOS or `ip` on Linux. Diagnostics normally do not
need sudo. Each check has a timeout, and failures do not stop the remaining checks.

Gateway reachability uses the default IPv4 gateway. A default tunnel route
without a gateway IP displays `tunnel route (utun4)` (with the actual interface
name), since there is no IP next hop to ping. Internet reachability,
average round-trip latency, and packet loss use four ICMP probes to `8.8.8.8`.
DNS health checks whether the system resolver can resolve `google.com`.
An ICMP-blocking firewall can make reachability fail even when web access works.
Missing tools or inconclusive checks display `unavailable`; total probe loss
reports `unreachable` and `100%` loss, with latency unavailable.

VPN status checks the route to `8.8.8.8`: a route through a `utun`, `tun`,
`tap`, `wg`, `ppp`, or `ipsec` interface reports `connected`; a physical interface
reports `disconnected`. Idle tunnel interfaces do not count. This remains a
heuristic: split-tunnel VPNs that do not route this target, VPNs with other
interface names, and non-VPN tunnels may not be classified accurately.
Route lookup failures report `unknown`.

Run tests with `go test ./...`. Like the server build, this requires the existing
libpcap development headers (macOS supplies libpcap; Linux typically uses
`libpcap-dev`). Completed diagnostics exit successfully even when checks fail;
Ping reply waits are bounded so blocked ICMP probes can report `100%` loss.
Inconclusive internet and failed DNS checks include their error reasons.
Invalid CLI arguments exit with status 2. Running without arguments starts the
web server as before.

To check a site reachable from your network, choose a custom target:

```bash
go run . diagnose --host tmcars.info
# Or: ./network-monitor diagnose --host tmcars.info
```

`--host` accepts a hostname or IPv4 address, without `https://`, a path, or a
port. Custom hostnames are resolved to IPv4 for ping and VPN route checks;
DNS health checks the supplied hostname. All internet measurements then refer
to that target. A site can work in a browser while blocking ICMP ping.




## Diagnostic source layout

```text
cmd/diagnose/
└── main.go                  # Standalone diagnose entry point
internal/diagnostic/
├── result.go                # Result type and summary formatting
├── diagnostic.go            # Network checks
└── diagnostic_test.go       # Diagnostic tests
internal/diagnosticcli/
├── cli.go                   # Shared CLI arguments and execution
└── cli_test.go              # CLI argument tests
```

Run the standalone command:

```bash
go run ./cmd/diagnose --host tmcars.info
```

The root command also continues to support `go run . diagnose` and
`go run . diagnose --host tmcars.info`.


## Application packages

```text
cmd/
├── diagnose/main.go       # Standalone diagnostics
└── server/main.go         # Standalone dashboard and API
internal/
├── diagnostic/           # Results, orchestration, gateway, DNS, ping, VPN
├── diagnosticcli/        # Diagnostic arguments and CLI tests
├── network/              # Device discovery, traffic, history, DNS proxy, limits
├── router/               # Router integrations
└── server/               # Server startup and HTTP routes
main.go                   # Existing root command dispatch
```

Start the server with `go run ./cmd/server` (packet capture may require sudo).
Existing `go run .` and `go run . diagnose` commands remain supported.

Diagnostics return `diagnostic.DiagnosticResult` independently of terminal output.
The result provides boolean connectivity fields, numeric `LatencyMS` and
`PacketLoss`, and JSON tags for serialization with `encoding/json`. Measurement
flags distinguish unavailable values from zero; status, error, and tunnel fields
preserve diagnostic details. Terminal formatting lives in `display.go`.
