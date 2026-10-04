package diagnostic

import (
	"context"
	"fmt"
	"net"
)

func resolveDiagnosticIPv4(ctx context.Context, host string) (string, error) {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() == nil {
			return "", fmt.Errorf("IPv4 target required")
		}
		return ip.String(), nil
	}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip4", host)
	if err != nil {
		return "", err
	}
	if len(addresses) == 0 {
		return "", fmt.Errorf("no IPv4 address for %s", host)
	}
	return addresses[0].String(), nil
}
