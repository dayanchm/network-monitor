package network

import (
	"os/exec"
	"strings"
)

type Device struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	Connected bool   `json:"connected"`
}

func ScanDevices() []Device {
	cmd := exec.Command("arp", "-a")
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")

	var devices []Device
	for _, line := range lines {
		if strings.Contains(line, "at") {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				ip := strings.Trim(fields[1], "()")
				mac := fields[3]
				hostname := GetHostname(ip)
				devices = append(devices, Device{
					IP:        ip,
					MAC:       mac,
					Hostname:  hostname,
					Connected: true,
				})
			}
		}
	}
	return devices
}

func GetHostname(ip string) string {
	cmd := exec.Command("nslookup", ip)
	output, _ := cmd.Output()
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "name =") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return strings.Trim(fields[2], ".")
			}
		}
	}
	return "unknown"
}
