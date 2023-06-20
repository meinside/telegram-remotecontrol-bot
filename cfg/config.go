package cfg

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

// constants for config
const (
	ConfigFilename = "config.json"
)

// Config struct for config file
type Config struct {
	APIToken                string   `json:"api_token"`
	AvailableIDs            []string `json:"available_ids"`
	ControllableServices    []string `json:"controllable_services,omitempty"`
	MountPoints             []string `json:"mount_points,omitempty"`
	MonitorInterval         int      `json:"monitor_interval"`
	TransmissionRPCPort     int      `json:"transmission_rpc_port,omitempty"`
	TransmissionRPCUsername string   `json:"transmission_rpc_username,omitempty"`
	TransmissionRPCPasswd   string   `json:"transmission_rpc_passwd,omitempty"`
	CLIPort                 int      `json:"cli_port"`
	IsVerbose               bool     `json:"is_verbose"`
}

// GetConfig reads config and return it
func GetConfig() (config Config, err error) {
	var execFilepath string
	if execFilepath, err = os.Executable(); err == nil {
		var file []byte
		if file, err = os.ReadFile(filepath.Join(filepath.Dir(execFilepath), ConfigFilename)); err == nil {
			var conf Config
			if err = json.Unmarshal(file, &conf); err == nil {
				// fallback values
				if config.TransmissionRPCPort <= 0 {
					config.TransmissionRPCPort = consts.DefaultTransmissionRPCPort
				}
				if config.MonitorInterval <= 0 {
					config.MonitorInterval = consts.DefaultMonitorIntervalSeconds
				}

				return conf, nil
			}
		}
	}

	return Config{}, err
}
