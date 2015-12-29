package helper

import (
	"fmt"
	"runtime"
	"time"
)

// get uptime of this bot in seconds
func GetUptime(launched time.Time) (uptime string) {
	now := time.Now()
	gap := now.Sub(launched)

	uptimeSeconds := int(gap.Seconds())
	numDays := uptimeSeconds / (60 * 60 * 24)
	numHours := (uptimeSeconds % (60 * 60 * 24)) / (60 * 60)

	return fmt.Sprintf("%d day(s) %d hour(s)", numDays, numHours)
}

func GetMemoryUsage() (usage string) {
	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)

	return fmt.Sprintf("Sys: %.1f MB, Heap: %.1f MB", float32(m.Sys)/1024/1024, float32(m.HeapAlloc)/1024/1024)
}
