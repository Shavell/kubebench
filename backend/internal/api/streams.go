package api

import "time"

// streamSettings holds the timings of the long-lived endpoints. Tests
// shorten them.
type streamSettings struct {
	// logHeartbeat is the interval of the comment lines that keep idle
	// log streams open through proxies.
	logHeartbeat time.Duration
}

func defaultStreamSettings() streamSettings {
	return streamSettings{
		logHeartbeat: 15 * time.Second,
	}
}
