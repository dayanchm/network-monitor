package network

import (
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func StartDNSProxy() {
	listenAddr := getEnv("DNS_LISTEN", ":53")
	upstream := getEnv("DNS_UPSTREAM", "1.1.1.1:53")

	handler := dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		clientIP, _, err := net.SplitHostPort(w.RemoteAddr().String())
		if err != nil {
			clientIP = w.RemoteAddr().String()
		}

		for _, question := range req.Question {
			domain := strings.TrimSuffix(question.Name, ".")
			if domain != "" {
				log.Printf("DNS query from %s: %s", clientIP, domain)
				SaveSiteVisit(clientIP, strings.ToLower(domain))
			}
		}

		client := &dns.Client{Timeout: 5 * time.Second}
		resp, _, err := client.Exchange(req, upstream)
		if err != nil {
			msg := new(dns.Msg)
			msg.SetRcode(req, dns.RcodeServerFailure)
			_ = w.WriteMsg(msg)
			log.Println("dns proxy upstream error:", err)
			return
		}

		_ = w.WriteMsg(resp)
	})

	go func() {
		server := &dns.Server{
			Addr:    listenAddr,
			Net:     "udp",
			Handler: handler,
		}
		log.Printf("DNS proxy started on udp %s, upstream %s", listenAddr, upstream)
		if err := server.ListenAndServe(); err != nil {
			log.Println("dns proxy udp error:", err)
		}
	}()

	go func() {
		server := &dns.Server{
			Addr:    listenAddr,
			Net:     "tcp",
			Handler: handler,
		}
		log.Printf("DNS proxy started on tcp %s, upstream %s", listenAddr, upstream)
		if err := server.ListenAndServe(); err != nil {
			log.Println("dns proxy tcp error:", err)
		}
	}()
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
