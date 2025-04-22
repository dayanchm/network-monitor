# 🛰️ Network Monitor

**Network Monitor** is a lightweight and extensible Go-based tool to **monitor local network activity**, **track device data usage**, and **enforce daily bandwidth limits** for devices connected to your Wi-Fi network.

---

## 🚀 Features

- 🔍 Scan all connected devices on the local network (IP, MAC, Hostname)
- 📡 Live traffic capture using `gopacket` and `pcap`
- 📊 Track and log per-device data usage (bytes in/out)
- 📏 Assign daily data limits (e.g., 1 GB/day)
- 🚫 Automatically trigger block logic when a device exceeds its quota *(you can define your own blocking logic)*
- 🌐 Simple REST API endpoints for easy integration

---

## 📦 Installation

```bash
git clone https://github.com/dayanchm/network-monitor
cd network-monitor
go build
./network-monitor