package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"network-monitor/internal/network"
	"network-monitor/internal/router"
	"os"
	"strings"
	"sync"
)

// Run starts the dashboard, API, DNS proxy and network tracking.
func Run() {
	historyDB, err := openConnectionHistory("devices.db")
	if err != nil {
		log.Fatalf("Connection history: %v", err)
	}
	defer historyDB.Close()
	go trackConnection(context.Background(), historyDB)
	http.HandleFunc("/api/connection-history", connectionHistoryHandler(historyDB))
	http.HandleFunc("/api/connection-analysis", connectionAnalysisHandler(historyDB))
	host := getEnv("HOST", "0.0.0.0")
	port := getEnv("PORT", "8888")
	iface := os.Getenv("NETWORK_INTERFACE")
	if iface == "" {
		var err error
		iface, err = network.DefaultInterface()
		if err != nil {
			log.Fatalf("No active network interface found. Set NETWORK_INTERFACE manually: %v", err)
		}
	}
	network.StartSiteTracking(iface)
	network.StartDNSProxy()
	network.ClearLoopbackSiteVisits()
	if localIP, err := network.InterfaceIPv4(iface); err == nil {
		log.Printf("Set tracked devices DNS server to: %s", localIP)
	}
	var scanMu sync.Mutex
	scanDevices := func() ([]network.Device, error) {
		scanMu.Lock()
		defer scanMu.Unlock()
		devices, err := network.ScanDevices()
		if err != nil {
			return nil, err
		}
		network.SaveDeviceHistory(devices)
		return devices, nil
	}
	go func() {
		log.Println("Startup device scan started")
		devices, err := scanDevices()
		if err != nil {
			log.Printf("Startup device scan: %v", err)
			return
		}
		log.Printf("Startup device scan completed: %d devices", len(devices))
	}()

	registerWebRoutes(http.DefaultServeMux)

	// list devices
	http.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/devices")
		devices, err := scanDevices()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		network.RespondWithDevices(w, devices)
	})

	http.HandleFunc("/api/known-devices", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/known-devices")
		current, err := scanDevices()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		seen := make(map[string]bool, len(current))
		for _, device := range current {
			seen[strings.ToLower(device.MAC)] = true
		}
		known := network.KnownDevices()
		for i := range known {
			known[i].Connected = seen[strings.ToLower(known[i].MAC)]
		}
		network.RespondWithDevices(w, known)
	})

	http.HandleFunc("/api/sites", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/sites")
		network.RespondWithDevices(w, network.SiteVisits())
	})

	http.HandleFunc("/api/traffic", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/traffic")
		w.Header().Set("Content-Type", "application/json")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		network.CaptureTraffic(iface, w, flusher)
	})

	http.HandleFunc("/api/limit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var limit network.UsageLimit
		if err := json.NewDecoder(r.Body).Decode(&limit); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		network.SetLimit(limit.MAC, limit.LimitGB)
		w.Write([]byte(`{"status":"ok"}`))
	})

	http.HandleFunc("/api/wifi/password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
			return
		}

		var change router.WifiPasswordChange
		if err := json.NewDecoder(r.Body).Decode(&change); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := router.ChangeWifiPassword(change); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := host + ":" + port
	log.Printf("Server started: http://%s", addr)
	log.Printf("Capturing traffic on interface: %s", iface)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
