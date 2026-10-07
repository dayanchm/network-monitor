package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"network-monitor/internal/ai"
	"network-monitor/internal/diagnostic"
	"os"
	"strings"
	"time"
)

func connectionAnalysisHandler(db *sql.DB) http.HandlerFunc {
	// Only one generation runs at a time, including across browser requests.
	busy := make(chan struct{}, 1)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "Invalid method", 405)
			return
		}
		var input struct {
			Start string `json:"start"`
			End   string `json:"end"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		start, err := time.Parse(time.RFC3339, input.Start)
		end, endErr := time.Parse(time.RFC3339, input.End)
		if err != nil || endErr != nil || !end.After(start) || end.Sub(start) > 48*time.Hour {
			http.Error(w, "Invalid time range", 400)
			return
		}
		select {
		case busy <- struct{}{}:
			defer func() { <-busy }()
		default:
			http.Error(w, "AI analysis already running", 429)
			return
		}
		var stamp int64
		var data string
		err = db.QueryRowContext(r.Context(), `SELECT time,result FROM connection_samples WHERE time >= ? AND time < ? ORDER BY time DESC LIMIT 1`, start.UnixMilli(), end.UnixMilli()).Scan(&stamp, &data)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "No measurements in this period", 404)
			return
		}
		if err != nil {
			http.Error(w, "Could not load measurement", 500)
			return
		}
		var result diagnostic.DiagnosticResult
		if json.Unmarshal([]byte(data), &result) != nil {
			http.Error(w, "Invalid stored measurement", 500)
			return
		}
		name := strings.ToLower(strings.TrimSpace(os.Getenv("AI_PROVIDER")))
		if name == "" {
			name = "ollama"
		}
		keys := map[string]string{"openai": "OPENAI_API_KEY", "anthropic": "ANTHROPIC_API_KEY", "gemini": "GEMINI_API_KEY", "deepseek": "DEEPSEEK_API_KEY"}
		cfg := ai.Config{Provider: name, Model: os.Getenv("AI_MODEL")}
		if key := keys[name]; key != "" {
			cfg.APIKey = os.Getenv(key)
		}
		if name == "ollama" {
			cfg.BaseURL = os.Getenv("OLLAMA_HOST")
			if cfg.Model == "" {
				cfg.Model = os.Getenv("OLLAMA_MODEL")
			}
		}
		provider, err := ai.NewProvider(cfg)
		if err != nil {
			http.Error(w, err.Error(), 503)
			return
		}
		analysis, err := provider.Analyze(r.Context(), result)
		if err != nil {
			http.Error(w, "AI analysis failed: "+err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(struct {
			Analysis string    `json:"analysis"`
			Time     time.Time `json:"time"`
			Provider string    `json:"provider"`
		}{analysis, time.UnixMilli(stamp).UTC(), name})
	}
}
