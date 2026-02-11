package main

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	st "github.com/meinside/rpi-tools/status"
)

// calculates uptime of this bot
func uptimeSince(launched time.Time) (uptime string) {
	now := time.Now()
	gap := now.Sub(launched)

	uptimeSeconds := int(gap.Seconds())
	numDays := uptimeSeconds / (60 * 60 * 24)
	numHours := (uptimeSeconds % (60 * 60 * 24)) / (60 * 60)

	return fmt.Sprintf("*%d* day(s) *%d* hour(s)", numDays, numHours)
}

// calculates memory usage of this bot
func memoryUsage() (usage string) {
	sys, heap := st.MemoryUsage()

	return fmt.Sprintf("sys *%.1f MB*, heap *%.1f MB*", float32(sys)/1024/1024, float32(heap)/1024/1024)
}

// calculates disk usage of the system (https://gist.github.com/lunny/9828326)
func diskUsage(additionalPaths []string) (usage string) {
	paths := []string{"/"}
	paths = append(paths, additionalPaths...)

	var lines []string
	for _, p := range paths {
		fs := syscall.Statfs_t{}
		if err := syscall.Statfs(p, &fs); err == nil {
			all := fs.Blocks * uint64(fs.Bsize)
			free := fs.Bavail * uint64(fs.Bsize)
			used := all - free

			lines = append(lines, fmt.Sprintf(
				"  %s  all *%.2f GB*, used *%.2f GB*, free *%.2f GB*",
				p,
				float64(all)/1024/1024/1024,
				float64(used)/1024/1024/1024,
				float64(free)/1024/1024/1024,
			))
		} else {
			lines = append(lines, fmt.Sprintf("%s: %s", p, err))
		}
	}

	return strings.Join(lines, "\n")
}

// removes markdown characters for avoiding
// 'Bad Request: Can't parse message text: Can't find end of the entity starting at byte offset ...' errors
// from the server
func removeMarkdownChars(original, replaceWith string) string {
	removed := strings.ReplaceAll(original, "*", replaceWith)
	removed = strings.ReplaceAll(removed, "_", replaceWith)
	removed = strings.ReplaceAll(removed, "`", replaceWith)
	return removed
}

// `systemctl status is-active`
func systemctlStatus(services []string) (statuses map[string]string, success bool) {
	statuses = make(map[string]string)

	args := []string{"systemctl", "is-active"}
	args = append(args, services...)

	output, _ := sudoRunCmd(args)
	for i, status := range strings.Split(output, "\n") {
		statuses[services[i]] = status
	}

	return statuses, true
}

// `systemctl start [service]`
func systemctlStart(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "start", service})
}

// `systemctl stop [service]`
func systemctlStop(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "stop", service})
}

// `systemctl restart [service]`
func systemctlRestart(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "restart", service})
}

// sudo run given command with parameters and return combined output
func sudoRunCmd(cmdAndParams []string) (string, error) {
	if len(cmdAndParams) < 1 {
		return "", fmt.Errorf("no command provided")
	}

	output, err := exec.Command("sudo", cmdAndParams...).CombinedOutput()
	return strings.TrimRight(string(output), "\n"), err
}

// returns a pointer to the given value
func ptr[T any](value T) *T {
	return &value
}
