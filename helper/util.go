package helper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/meinside/rpi-tools/status"
)

// constants for config
const (
	ConfigFilename = "config.json"
)

// Config struct for config file
type Config struct {
	APIToken                string   `json:"api_token"`
	AvailableIds            []string `json:"available_ids"`
	ControllableServices    []string `json:"controllable_services,omitempty"`
	MountPoints             []string `json:"mount_points,omitempty"`
	MonitorInterval         int      `json:"monitor_interval"`
	TransmissionRPCPort     int      `json:"transmission_rpc_port,omitempty"`
	TransmissionRPCUsername string   `json:"transmission_rpc_username,omitempty"`
	TransmissionRPCPasswd   string   `json:"transmission_rpc_passwd,omitempty"`
	CliPort                 int      `json:"cli_port"`
	IsVerbose               bool     `json:"is_verbose"`
}

// GetConfig reads config and return it
func GetConfig() (config Config, err error) {
	var execFilepath string
	if execFilepath, err = os.Executable(); err == nil {
		var file []byte
		if file, err = ioutil.ReadFile(filepath.Join(filepath.Dir(execFilepath), ConfigFilename)); err == nil {
			var conf Config
			if err = json.Unmarshal(file, &conf); err == nil {
				return conf, nil
			}
		}
	}

	return Config{}, err
}

// GetUptime calculates uptime of this bot
func GetUptime(launched time.Time) (uptime string) {
	now := time.Now()
	gap := now.Sub(launched)

	uptimeSeconds := int(gap.Seconds())
	numDays := uptimeSeconds / (60 * 60 * 24)
	numHours := (uptimeSeconds % (60 * 60 * 24)) / (60 * 60)

	return fmt.Sprintf("*%d* day(s) *%d* hour(s)", numDays, numHours)
}

// GetMemoryUsage calculates memory usage of this bot
func GetMemoryUsage() (usage string) {
	sys, heap := status.MemoryUsage()

	return fmt.Sprintf("sys *%.1f MB*, heap *%.1f MB*", float32(sys)/1024/1024, float32(heap)/1024/1024)
}

// GetDiskUsage calculates disk usage of the system (https://gist.github.com/lunny/9828326)
func GetDiskUsage(additionalPaths []string) (usage string) {
	paths := []string{"/"}
	for _, p := range additionalPaths {
		paths = append(paths, p)
	}

	var lines []string
	for _, p := range paths {
		fs := syscall.Statfs_t{}
		if err := syscall.Statfs(p, &fs); err == nil {
			all := fs.Blocks * uint64(fs.Bsize)
			free := fs.Bfree * uint64(fs.Bsize)
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

// RemoveMarkdownChars removes markdown characters for avoiding
// 'Bad Request: Can't parse message text: Can't find end of the entity starting at byte offset ...' error
// from the server
func RemoveMarkdownChars(original, replaceWith string) string {
	removed := strings.Replace(original, "*", replaceWith, -1)
	removed = strings.Replace(removed, "_", replaceWith, -1)
	removed = strings.Replace(removed, "`", replaceWith, -1)
	return removed
}

// Sudo run given command with parameters and return combined output
func sudoRunCmd(cmdAndParams []string) (string, error) {
	if len(cmdAndParams) < 1 {
		return "", fmt.Errorf("no command provided")
	}

	output, err := exec.Command("sudo", cmdAndParams...).CombinedOutput()
	return strings.TrimRight(string(output), "\n"), err
}

// SystemctlStatus runs `systemctl status is-active`
func SystemctlStatus(services []string) (statuses map[string]string, success bool) {
	statuses = make(map[string]string)

	args := []string{"systemctl", "is-active"}
	args = append(args, services...)

	output, _ := sudoRunCmd(args)
	for i, status := range strings.Split(output, "\n") {
		statuses[services[i]] = status
	}

	return statuses, true
}

// SystemctlStart runs `systemctl start [service]`
func SystemctlStart(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "start", service})
}

// SystemctlStop runs `systemctl stop [service]`
func SystemctlStop(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "stop", service})
}

// SystemctlRestart runs `systemctl restart [service]`
func SystemctlRestart(service string) (message string, err error) {
	return sudoRunCmd([]string{"systemctl", "restart", service})
}
