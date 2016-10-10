package helper

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	// constants for config
	ConfigFilename = "../config.json"
)

// struct for config file
type Config struct {
	ApiToken                string   `json:"api_token"`
	AvailableIds            []string `json:"available_ids"`
	ControllableServices    []string `json:"controllable_services,omitempty"`
	MountPoints             []string `json:"mount_points,omitempty"`
	MonitorInterval         int      `json:"monitor_interval"`
	TransmissionRpcPort     int      `json:"transmission_rpc_port,omitempty"`
	TransmissionRpcUsername string   `json:"transmission_rpc_username,omitempty"`
	TransmissionRpcPasswd   string   `json:"transmission_rpc_passwd,omitempty"`
	CliPort                 int      `json:"cli_port"`
	IsVerbose               bool     `json:"is_verbose"`
}

// Read config
func GetConfig() (config Config, err error) {
	_, filename, _, _ := runtime.Caller(0) // = __FILE__

	if file, err := ioutil.ReadFile(filepath.Join(path.Dir(filename), ConfigFilename)); err == nil {
		var conf Config
		if err := json.Unmarshal(file, &conf); err == nil {
			return conf, nil
		} else {
			return Config{}, err
		}
	} else {
		return Config{}, err
	}
}

// get uptime of this bot in seconds
func GetUptime(launched time.Time) (uptime string) {
	now := time.Now()
	gap := now.Sub(launched)

	uptimeSeconds := int(gap.Seconds())
	numDays := uptimeSeconds / (60 * 60 * 24)
	numHours := (uptimeSeconds % (60 * 60 * 24)) / (60 * 60)

	return fmt.Sprintf("*%d* day(s) *%d* hour(s)", numDays, numHours)
}

// get memory usage
func GetMemoryUsage() (usage string) {
	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)

	return fmt.Sprintf("Sys: *%.1f MB*, Heap: *%.1f MB*", float32(m.Sys)/1024/1024, float32(m.HeapAlloc)/1024/1024)
}

// get disk usage (https://gist.github.com/lunny/9828326)
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

// XXX - remove markdown characters for avoiding
// 'Bad Request: Can't parse message text: Can't find end of the entity starting at byte offset ...' error
// from the server
func RemoveMarkdownChars(original, replaceWith string) string {
	removed := strings.Replace(original, "*", replaceWith, -1)
	removed = strings.Replace(removed, "_", replaceWith, -1)
	removed = strings.Replace(removed, "`", replaceWith, -1)
	return removed
}
