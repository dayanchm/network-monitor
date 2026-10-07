package network

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type Device struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	Connected bool   `json:"connected"`
}

func ScanDevices() ([]Device, error) {
	iface, err := DefaultInterface()
	if err != nil {
		return nil, err
	}
	devices, err := scanWithARPScan(iface)
	if err != nil {
		return nil, err
	}
	refreshDHCPLeases()
	for i := range devices {
		devices[i] = newDevice(devices[i].IP, devices[i].MAC)
	}
	sort.Slice(devices, func(i, j int) bool {
		return ipLess(devices[i].IP, devices[j].IP)
	})
	return devices, nil
}

func scanWithARPScan(ifaceName string) ([]Device, error) {
	arpScanPath, err := arpScanCommand()
	if err != nil {
		return nil, fmt.Errorf("active scan requires arp-scan; install it with brew install arp-scan: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, arpScanPath, "--interface="+ifaceName, "--plain", "--ignoredups", "--localnet").CombinedOutput()
	if err != nil {
		log.Printf("arp-scan failed on %s: %v: %s", ifaceName, err, strings.TrimSpace(string(output)))
		return nil, fmt.Errorf("active scan failed on %s; check packet-capture permissions: %w", ifaceName, err)
	}

	seen := make(map[string]bool)
	devices := []Device{}
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := fields[0]
		mac := fields[1]
		if !isDeviceIP(ip) || !isMAC(mac) || seen[ip] {
			continue
		}
		seen[ip] = true
		devices = append(devices, Device{IP: ip, MAC: mac, Connected: true})
	}
	return devices, nil
}

func arpScanCommand() (string, error) {
	if path, err := exec.LookPath("arp-scan"); err == nil {
		return path, nil
	}
	for _, path := range []string{"/opt/homebrew/bin/arp-scan", "/usr/local/bin/arp-scan"} {
		if _, err := exec.Command(path, "--version").Output(); err == nil {
			return path, nil
		}
	}
	return "", exec.ErrNotFound
}

func scanWithARPCache() []Device {
	cmd := exec.Command("arp", "-an")
	output, _ := cmd.Output()

	seen := make(map[string]bool)
	var devices []Device
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "at") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				ip := strings.Trim(fields[1], "()")
				if !isDeviceIP(ip) || seen[ip] {
					continue
				}
				seen[ip] = true
				mac := fields[3]
				if !isMAC(mac) {
					continue
				}
				devices = append(devices, newDevice(ip, mac))
			}
		}
	}
	return devices
}

func newDevice(ip, mac string) Device {

	MACMap[ip] = mac
	return Device{
		IP:        ip,
		MAC:       mac,
		Hostname:  GetHostname(ip, mac),
		Connected: true,
	}

}
func isDeviceIP(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil || ip.To4() == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return !strings.HasSuffix(value, ".255")
}

func isMAC(value string) bool {
	_, err := net.ParseMAC(value)
	return err == nil
}

func ipLess(left, right string) bool {
	leftIP := net.ParseIP(left).To4()
	rightIP := net.ParseIP(right).To4()
	if leftIP == nil || rightIP == nil {
		return left < right
	}
	for i := 0; i < net.IPv4len; i++ {
		if leftIP[i] == rightIP[i] {
			continue
		}
		return leftIP[i] < rightIP[i]
	}
	return false
}

func GetHostname(ip, mac string) string {

	if hostname := lookupOwnHostname(ip); hostname != "" {
		return hostname
	}
	if hostname := DHCPHostname(ip, mac); hostname != "" {
		return hostname
	}
	if hostname := lookupReverseDNS(ip); hostname != "" {
		return hostname
	}
	if hostname := LookupMDNSHostname(ip); hostname != "" {
		return hostname
	}
	if isDefaultGateway(ip) {
		return "gateway"
	}
	return "unknown"

}

func lookupReverseDNS(ip string) string {
	names, err := net.LookupAddr(ip)
	if err != nil || len(names) == 0 {
		return ""
	}

	return strings.TrimSuffix(names[0], ".")
}

func lookupOwnHostname(ip string) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.String() == ip {
				if hostname, err := osHostname(); err == nil && hostname != "" {
					return hostname
				}
			}
		}
	}
	return ""
}

func osHostname() (string, error) {
	if output, err := exec.Command("scutil", "--get", "LocalHostName").Output(); err == nil {
		hostname := strings.TrimSpace(string(output))
		if hostname != "" {
			return hostname, nil
		}
	}
	output, err := exec.Command("hostname").Output()
	return strings.TrimSpace(string(output)), err
}

func isDefaultGateway(ip string) bool {
	output, err := exec.Command("route", "-n", "get", "default").Output()
	if err == nil && outputHasGateway(output, ip) {
		return true
	}

	output, err = exec.Command("netstat", "-rn", "-f", "inet").Output()
	if err == nil && outputHasDefaultGateway(output, ip) {
		return true
	}

	return false
}

func outputHasGateway(output []byte, ip string) bool {
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "gateway:" && fields[1] == ip {
			return true
		}
	}
	return false
}

func outputHasDefaultGateway(output []byte, ip string) bool {
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "default" && fields[1] == ip {
			return true
		}
	}
	return false
}
