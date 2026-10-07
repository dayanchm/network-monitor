package network

import (
	"database/sql"
	"log"
	"net"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const databasePath = "devices.db"

type KnownDevice struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	Connected bool   `json:"connected"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
}

type SiteVisit struct {
	DeviceIP  string `json:"device_ip"`
	MAC       string `json:"mac"`
	Hostname  string `json:"hostname"`
	Domain    string `json:"domain"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	Count     int    `json:"count"`
}

func SaveDeviceHistory(devices []Device) {
	db, err := openHistoryDB()
	if err != nil {
		log.Println("device history open error:", err)
		return
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	for _, device := range devices {
		_, err := db.Exec(`
			INSERT INTO device_history (mac, ip, hostname, first_seen, last_seen)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(mac) DO UPDATE SET
				ip = excluded.ip,
				hostname = CASE
					WHEN excluded.hostname IN ('', 'unknown', 'gateway')
						AND device_history.hostname NOT IN ('', 'unknown', 'gateway')
					THEN device_history.hostname
					ELSE excluded.hostname
				END,
				last_seen = excluded.last_seen
		`, device.MAC, device.IP, device.Hostname, now, now)
		if err != nil {
			log.Println("device history save error:", err)
		}
	}
}

func KnownDevices() []KnownDevice {
	db, err := openHistoryDB()
	if err != nil {
		log.Println("device history open error:", err)
		return []KnownDevice{}
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT ip, mac, hostname, first_seen, last_seen
		FROM device_history
		ORDER BY last_seen DESC, ip ASC
	`)
	if err != nil {
		log.Println("device history query error:", err)
		return []KnownDevice{}
	}
	defer rows.Close()

	devices := []KnownDevice{}
	for rows.Next() {
		var device KnownDevice
		if err := rows.Scan(&device.IP, &device.MAC, &device.Hostname, &device.FirstSeen, &device.LastSeen); err != nil {
			log.Println("device history scan error:", err)
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

func SaveSiteVisit(deviceIP, domain string) {
	if deviceIP == "" || domain == "" {
		return
	}
	if ip := net.ParseIP(deviceIP); ip == nil || ip.IsLoopback() {
		return
	}

	db, err := openHistoryDB()
	if err != nil {
		log.Println("site history open error:", err)
		return
	}
	defer db.Close()

	now := time.Now().Format(time.RFC3339)
	mac := MACMap[deviceIP]
	hostname := GetHostname(deviceIP, mac)
	_, err = db.Exec(`
		INSERT INTO site_history (device_ip, mac, hostname, domain, first_seen, last_seen, count)
		VALUES (?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(device_ip, domain) DO UPDATE SET
			mac = excluded.mac,
			hostname = excluded.hostname,
			last_seen = excluded.last_seen,
			count = count + 1
	`, deviceIP, mac, hostname, domain, now, now)
	if err != nil {
		log.Println("site history save error:", err)
	}
}

func ClearLoopbackSiteVisits() {
	db, err := openHistoryDB()
	if err != nil {
		log.Println("site history open error:", err)
		return
	}
	defer db.Close()

	if _, err := db.Exec(`DELETE FROM site_history WHERE device_ip = '127.0.0.1' OR device_ip = '::1'`); err != nil {
		log.Println("site history cleanup error:", err)
	}
}

func SiteVisits() []SiteVisit {
	db, err := openHistoryDB()
	if err != nil {
		log.Println("site history open error:", err)
		return []SiteVisit{}
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT device_ip, mac, hostname, domain, first_seen, last_seen, count
		FROM site_history
		ORDER BY last_seen DESC, count DESC
		LIMIT 300
	`)
	if err != nil {
		log.Println("site history query error:", err)
		return []SiteVisit{}
	}
	defer rows.Close()

	visits := []SiteVisit{}
	for rows.Next() {
		var visit SiteVisit
		if err := rows.Scan(&visit.DeviceIP, &visit.MAC, &visit.Hostname, &visit.Domain, &visit.FirstSeen, &visit.LastSeen, &visit.Count); err != nil {
			log.Println("site history scan error:", err)
			continue
		}
		visits = append(visits, visit)
	}
	return visits
}

func openHistoryDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", databasePath)
	if err != nil {
		return nil, err
	}
	if err := ensureHistorySchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ensureHistorySchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS device_history (
			mac TEXT PRIMARY KEY,
			ip TEXT NOT NULL,
			hostname TEXT NOT NULL,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS site_history (
			device_ip TEXT NOT NULL,
			mac TEXT NOT NULL,
			hostname TEXT NOT NULL,
			domain TEXT NOT NULL,
			first_seen TEXT NOT NULL,
			last_seen TEXT NOT NULL,
			count INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (device_ip, domain)
		);
	`)
	return err
}
