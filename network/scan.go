package network

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type TrafficPacket struct {
	Timestamp string `json:"timestamp"`
	SourceIP  string `json:"source_ip"`
	DestIP    string `json:"dest_ip"`
	Protocol  string `json:"protocol"`
}

var UsageMap = make(map[string]int64) // IP → byte sayısı
var MACMap = make(map[string]string)  // IP → MAC eşlemesi

func CaptureTraffic(w io.Writer, flusher http.Flusher) {
	handle, err := pcap.OpenLive("en0", 65536, true, pcap.BlockForever)
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

			// ENGELLEME noktası:
			// exec.Command("sudo", "iptables", "-A", "INPUT", "-m", "mac", "--mac-source", mac, "-j", "DROP").Run()
		}

		packetData := TrafficPacket{
			Timestamp: time.Now().Format(time.RFC3339),
			SourceIP:  ip,
			DestIP:    networkLayer.NetworkFlow().Dst().String(),
			Protocol:  packet.TransportLayer().LayerType().String(),
		}
		encoder.Encode(packetData)
		flusher.Flush()
	}
}
