package network

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type DHCPLease struct {
	Hostname string
	IP       string
	MAC      string
}

func FetchDHCPLeases() ([]DHCPLease, error) {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get("http://192.168.1.1/dhcptbl.htm")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DHCP page returned status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	var leases []DHCPLease

	doc.Find("table tr").Each(func(_ int, row *goquery.Selection) {
		var cols []string

		row.Find("td").Each(func(_ int, cell *goquery.Selection) {
			cols = append(cols, strings.TrimSpace(cell.Text()))
		})

		// İsim, IP, MAC, Son Kullanma, Tür
		if len(cols) < 3 {
			return
		}

		ip := cols[1]
		mac := strings.ToLower(cols[2])

		if !isDeviceIP(ip) || !isMAC(mac) {
			return
		}

		leases = append(leases, DHCPLease{
			Hostname: cols[0],
			IP:       ip,
			MAC:      mac,
		})
	})

	return leases, nil
}

func DHCPHostname(ip, mac string) string {
	leases, err := FetchDHCPLeases()
	if err != nil {
		return ""
	}

	for _, lease := range leases {
		if lease.IP == ip {
			return lease.Hostname
		}

		if mac != "" && strings.EqualFold(lease.MAC, mac) {
			return lease.Hostname
		}
	}

	return ""
}
