package diagnostic

func classifyLatency(ms float64) string {
	switch {
	case ms < 50:
		return "low"
	case ms < 100:
		return "moderate"
	case ms < 200:
		return "high"
	default:
		return "very_high"
	}
}
