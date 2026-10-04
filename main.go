package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"network-monitor/network"
	"network-monitor/network/router"
	"os"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		host, err := parseDiagnoseArgs(os.Args[1:], os.Stderr)
		if err != nil {
			if err == flag.ErrHelp {
				return
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		var result network.Diagnostics
		if host == "" {
			result = network.Diagnose(context.Background())
		} else {
			result = network.DiagnoseHost(context.Background(), host)
		}
		if err := result.Print(os.Stdout); err != nil {
			log.Fatal(err)
		}

		return
	}

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "index.html")
	})

	// list devices
	http.HandleFunc("/api/devices", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/devices")
		devices := network.ScanDevices()
		network.SaveDeviceHistory(devices)
		network.RespondWithDevices(w, devices)
	})

	http.HandleFunc("/api/known-devices", func(w http.ResponseWriter, r *http.Request) {
		log.Println("GET /api/known-devices")
		network.RespondWithDevices(w, network.KnownDevices())
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

func parseDiagnoseArgs(args []string, output io.Writer) (string, error) {
	if len(args) == 0 || args[0] != "diagnose" {
		return "", fmt.Errorf("Usage: network-monitor [diagnose [--host hostname]]")
	}
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(output)
	host := flags.String("host", "", "IPv4 address or hostname to diagnose (no URL or port)")
	if err := flags.Parse(args[1:]); err != nil {
		return "", err
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected argument: %s", flags.Arg(0))
	}
	supplied := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "host" {
			supplied = true
		}
	})
	if supplied {
		if *host == "" || strings.HasPrefix(*host, "-") || strings.ContainsAny(*host, "/: \t\r\n") {
			return "", fmt.Errorf("--host requires a hostname or IPv4 address, without a URL or port")
		}
	}
	return *host, nil
}
