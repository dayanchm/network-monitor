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
