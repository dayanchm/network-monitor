package network

import (
	"strings"
	"testing"
)

func TestParseDHCPLeases(t *testing.T) {
	html := `<table><tr><td>Hostname</td><td>IP Address</td><td>MAC</td></tr><tr><td>My Phone</td><td>192.168.1.2</td><td>AA-BB-CC-DD-EE-FF</td><td>Auto</td></tr></table>`
	leases, err := parseDHCPLeases(strings.NewReader(html))
	if err != nil || len(leases) != 1 {
		t.Fatalf("leases=%v error=%v", leases, err)
	}
	if leases[0].Hostname != "My Phone" || leases[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("unexpected lease: %+v", leases[0])
	}
}

func TestParseDHCPLoginPage(t *testing.T) {
	_, err := parseDHCPLeases(strings.NewReader(`<html><form><input type="password"></form></html>`))
	if err == nil {
		t.Fatal("expected error for login page")
	}
}
