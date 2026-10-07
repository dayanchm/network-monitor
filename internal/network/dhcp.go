package network

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type DHCPLease struct {
	Hostname string
	IP       string
	MAC      string
}

var dhcpCache struct {
	sync.Mutex
	leases  []DHCPLease
	checked time.Time
}

func refreshDHCPLeases() {
	dhcpCache.Lock()
	defer dhcpCache.Unlock()
	leases, err := FetchDHCPLeases()
	dhcpCache.checked = time.Now()
	dhcpCache.leases = leases
	if err != nil {
		log.Printf("Router DHCP names unavailable: %v", err)
	}
}

func FetchDHCPLeases() ([]DHCPLease, error) {
	client := &http.Client{
		Timeout:       3 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	pageURL := os.Getenv("ROUTER_DHCP_URL")
	if pageURL == "" {
		pageURL = "http://192.168.1.1/dhcptbl.htm"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid ROUTER_DHCP_URL")
	}
	if username := os.Getenv("ROUTER_USERNAME"); username != "" {
		req.SetBasicAuth(username, os.Getenv("ROUTER_PASSWORD"))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DHCP request failed; check ROUTER_DHCP_URL and router access")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DHCP page returned status %d; check page address and ROUTER_USERNAME/ROUTER_PASSWORD", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("DHCP page exceeds 1 MiB")
	}
	return parseDHCPLeases(strings.NewReader(string(data)))
}

func parseDHCPLeases(reader io.Reader) ([]DHCPLease, error) {
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, err
	}

	var leases []DHCPLease

	doc.Find("table tr").Each(func(_ int, row *goquery.Selection) {
		var cols []string

		row.ChildrenFiltered("td").Each(func(_ int, cell *goquery.Selection) {
			cols = append(cols, strings.TrimSpace(cell.Text()))
		})

		// İsim, IP, MAC, Son Kullanma, Tür
		if len(cols) < 3 {
			return
		}

		ip := cols[1]
		parsedMAC, macErr := net.ParseMAC(strings.ReplaceAll(cols[2], "-", ":"))
		mac := parsedMAC.String()

		if !isDeviceIP(ip) || macErr != nil {
			return
		}

		leases = append(leases, DHCPLease{
			Hostname: cols[0],
			IP:       ip,
			MAC:      mac,
		})
	})

	if len(leases) == 0 {
		return nil, fmt.Errorf("no DHCP client rows found; verify ROUTER_DHCP_URL points to the client table, not the login page")
	}
	return leases, nil
}

func DHCPHostname(ip, mac string) string {
	dhcpCache.Lock()
	needsRefresh := time.Since(dhcpCache.checked) > 30*time.Second
	dhcpCache.Unlock()
	if needsRefresh {
		refreshDHCPLeases()
	}
	dhcpCache.Lock()
	leases := append([]DHCPLease(nil), dhcpCache.leases...)
	dhcpCache.Unlock()

	for _, lease := range leases {
		if (mac != "" && strings.EqualFold(lease.MAC, mac)) || (mac == "" && lease.IP == ip) {
			name := strings.TrimSpace(lease.Hostname)
			if name == "" || name == "-" || name == "*" || strings.EqualFold(name, "unknown") {
				return ""
			}
			return name
		}
	}

	return ""
}
