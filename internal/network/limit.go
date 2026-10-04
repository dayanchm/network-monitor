package network

type UsageLimit struct {
	MAC     string  `json:"mac"`
	LimitGB float64 `json:"limit_gb"`
}

var UserLimits = make(map[string]float64)

func SetLimit(mac string, gb float64) {
	UserLimits[mac] = gb
}

func IsLimitExceeded(mac string, usedBytes int64) bool {
	limit, ok := UserLimits[mac]
	if !ok {
		return false
	}
	return float64(usedBytes)/(1024*1024*1024) > limit
}
