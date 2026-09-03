package network

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type TrafficPacket struct {
	Timestamp string `json:"timestamp"`
	SourceIP  string `json:"source_ip"`
	DestIP    string `json:"dest_ip"`
	Protocol  string `json:"protocol"`
}

var UsageMap = make(map[string]int64) // IP → byte
var MACMap = make(map[string]string)  // IP → MAC

func DefaultInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if ok && ipNet.IP.To4() != nil {
				return iface.Name, nil
			}
		}
	}

	return "", errors.New("active IPv4 network interface not found")
}

func InterfaceIPv4(ifaceName string) (string, error) {
	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}

	return "", errors.New("interface IPv4 address not found")
}

func CaptureTraffic(iface string, w io.Writer, flusher http.Flusher) {
	handle, err := pcap.OpenLive(iface, 65536, true, pcap.BlockForever)
	if err != nil {
		log.Println("pcap error:", err)
		return
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	encoder := json.NewEncoder(w)

	for packet := range packetSource.Packets() {
		networkLayer := packet.NetworkLayer()
		if networkLayer == nil {
			continue
		}

		ip := networkLayer.NetworkFlow().Src().String()
		bytes := int64(len(packet.Data()))
		UsageMap[ip] += bytes

		// Limit kontrol
		mac := MACMap[ip]
		if IsLimitExceeded(mac, UsageMap[ip]) {
			log.Println("Limit aşıldı:", ip, mac)

			// exec.Command("sudo", "iptables", "-A", "INPUT", "-m", "mac", "--mac-source", mac, "-j", "DROP").Run()
		}

		packetData := TrafficPacket{
			Timestamp: time.Now().Format(time.RFC3339),
			SourceIP:  ip,
			DestIP:    networkLayer.NetworkFlow().Dst().String(),
			Protocol:  "unknown",
		}
		if transportLayer := packet.TransportLayer(); transportLayer != nil {
			packetData.Protocol = transportLayer.LayerType().String()
		}
		saveDNSQueries(packet, ip)
		encoder.Encode(packetData)
		flusher.Flush()
	}
}

func StartSiteTracking(iface string) {
	go func() {
		handle, err := pcap.OpenLive(iface, 65536, true, pcap.BlockForever)
		if err != nil {
			log.Println("site tracking pcap error:", err)
			return
		}
		defer handle.Close()

		if err := handle.SetBPFFilter("udp port 53 or tcp port 53"); err != nil {
			log.Println("site tracking filter error:", err)
			return
		}

		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			networkLayer := packet.NetworkLayer()
			if networkLayer == nil {
				continue
			}
			saveDNSQueries(packet, networkLayer.NetworkFlow().Src().String())
		}
	}()
}

func saveDNSQueries(packet gopacket.Packet, sourceIP string) {
	dnsLayer := packet.Layer(layers.LayerTypeDNS)
	if dnsLayer == nil {
		return
	}

	dns, ok := dnsLayer.(*layers.DNS)
	if !ok || dns.QR {
		return
	}

	for _, question := range dns.Questions {
		domain := strings.TrimSuffix(string(question.Name), ".")
		if domain != "" {
			SaveSiteVisit(sourceIP, strings.ToLower(domain))
		}
	}
}
