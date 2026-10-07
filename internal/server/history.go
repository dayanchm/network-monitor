package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"network-monitor/internal/diagnostic"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type connectionSample struct {
	Time   time.Time                   `json:"time"`
	Status string                      `json:"status"`
	Result diagnostic.DiagnosticResult `json:"result"`
}

func connectionStatus(r diagnostic.DiagnosticResult) string {
	if r.InternetStatus == "unreachable" {
		return "unreachable"
	}
	if r.InternetStatus != "reachable" {
		return "unknown"
	}
	if !r.DNSHealthy || (r.PacketLossMeasured && r.PacketLoss > 0) || (r.LatencyMeasured && r.LatencyMS >= 150) {
		return "degraded"
	}
	return "healthy"
}

func openConnectionHistory(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS connection_samples (time INTEGER PRIMARY KEY, result TEXT NOT NULL)`)
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func saveConnectionSample(db *sql.DB, at time.Time, result diagnostic.DiagnosticResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT OR REPLACE INTO connection_samples(time,result) VALUES (?,?)`, at.UnixMilli(), string(data))
	return err
}

func connectionHistoryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Invalid method", 405)
			return
		}
		start, err := time.Parse(time.RFC3339, r.URL.Query().Get("start"))
		end, endErr := time.Parse(time.RFC3339, r.URL.Query().Get("end"))
		if err != nil || endErr != nil || !end.After(start) || end.Sub(start) > 48*time.Hour {
			http.Error(w, "start/end must specify a range of at most 48 hours", 400)
			return
		}
		rows, err := db.QueryContext(r.Context(), `SELECT time,result FROM connection_samples WHERE time >= ? AND time < ? ORDER BY time`, start.UnixMilli(), end.UnixMilli())
		if err != nil {
			http.Error(w, "Could not load connection history", 500)
			return
		}
		defer rows.Close()
		samples := []connectionSample{}
		for rows.Next() {
			var stamp int64
			var data string
			var result diagnostic.DiagnosticResult
			if err := rows.Scan(&stamp, &data); err != nil {
				http.Error(w, "Could not read connection history", 500)
				return
			}
			if err := json.Unmarshal([]byte(data), &result); err != nil {
				http.Error(w, "Invalid stored measurement", 500)
				return
			}
			samples = append(samples, connectionSample{time.UnixMilli(stamp).UTC(), connectionStatus(result), result})
		}
		if rows.Err() != nil {
			http.Error(w, "Could not read connection history", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(samples)
	}
}

func trackConnection(ctx context.Context, db *sql.DB) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		at := time.Now()
		result := diagnostic.Diagnose(ctx)
		if ctx.Err() != nil {
			return
		}
		if err := saveConnectionSample(db, at, result); err != nil {
			log.Println("connection history:", err)
		}
		if _, err := db.Exec(`DELETE FROM connection_samples WHERE time < ?`, at.AddDate(0, 0, -30).UnixMilli()); err != nil {
			log.Println("connection retention:", fmt.Errorf("cleanup: %w", err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
