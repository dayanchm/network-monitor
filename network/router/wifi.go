package router

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type WifiPasswordChange struct {
	RouterIP string `json:"router_ip"`
	Username string `json:"username"`
	Admin    string `json:"admin"`
	Password string `json:"password"`
}

func ChangeWifiPassword(change WifiPasswordChange) error {
	change.RouterIP = strings.TrimSpace(change.RouterIP)
	change.Username = strings.TrimSpace(change.Username)
	change.Admin = strings.TrimSpace(change.Admin)
	change.Password = strings.TrimSpace(change.Password)

	if change.RouterIP == "" {
		return errors.New("router IP is required")
	}
	if change.Username == "" {
		return errors.New("admin username is required")
	}
	if change.Admin == "" {
		return errors.New("admin password is required")
	}
	if len(change.Password) < 8 {
		return errors.New("wifi password must be at least 8 characters")
	}

	return changeLegacyTPLinkPassword(change)
}

func changeLegacyTPLinkPassword(change WifiPasswordChange) error {
	baseURL := "http://" + strings.TrimPrefix(strings.TrimPrefix(change.RouterIP, "http://"), "https://")
	endpoint := baseURL + "/userRpm/WlanSecurityRpm.htm"

	values := url.Values{}
	values.Set("pskSecOpt", "3")
	values.Set("pskCipher", "3")
	values.Set("pskSecret", change.Password)
	values.Set("interval", "0")
	values.Set("Save", "Save")

	requestURL := endpoint + "?" + values.Encode()
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(change.Username, change.Admin)
	req.Header.Set("Referer", baseURL+"/userRpm/WlanSecurityRpm.htm")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("router request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return errors.New("router admin login failed")
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("router returned status %d", resp.StatusCode)
	}

	return nil
}
