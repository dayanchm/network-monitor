package network

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

func LookupMDNSHostname(ip string) string {
	target := net.ParseIP(ip)
	if target == nil {
		return ""
	}
	services := []string{
		"_workstation._tcp",
		"_http._tcp",
		"_airplay._tcp",
		"_googlecast._tcp",
		"_printer._tcp",
	}

	for _, service := range services {
		if hostname := browseServiceForIP(service, target); hostname != "" {
			return hostname
		}
	}

	return ""
}

func browseServiceForIP(service string, target net.IP) string {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry)

	var hostname string

	go func() {
		for entry := range entries {
			for _, addr := range entry.AddrIPv4 {
				if addr.Equal(target) && entry.HostName != "" {
					hostname = strings.TrimSuffix(entry.HostName, ".")
					cancel()
					return
				}
			}
		}
	}()

	if err := resolver.Browse(ctx, service, "local.", entries); err != nil {
		return ""
	}

	<-ctx.Done()

	return hostname
}
