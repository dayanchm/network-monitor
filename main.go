package main

import (
	"encoding/json"
	"log"
	"net/http"
	"network-monitor/network"
)

func main() {
	// Cihazları listeleme
	http.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/devices")
		devices := network.ScanDevices()
		network.RespondWithDevices(w, devices)
	})

	http.HandleFunc("/api/traffic", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/traffic")
		w.Header().Set("Content-Type", "application/json")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		network.CaptureTraffic(w, flusher)
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

	log.Println("Server started: http://localhost:8888")
	log.Fatal(http.ListenAndServe(":8888", nil))
}
