package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

// constants for config
const (
	AppName        = "telegram-remotecontrol-bot"
	configFilename = "config.json"
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

// get .config directory path
func GetConfigDir() (configDir string, err error) {
	// https://xdgbasedirectoryspecification.com
	configDir = os.Getenv("XDG_CONFIG_HOME")

	// If the value of the environment variable is unset, empty, or not an absolute path, use the default
	if configDir == "" || configDir[0:1] != "/" {
		var homeDir string
		if homeDir, err = os.UserHomeDir(); err == nil {
			configDir = filepath.Join(homeDir, ".config", AppName)
		}
	} else {
		configDir = filepath.Join(configDir, AppName)
	}

	return configDir, err
}

// GetConfig reads config and return it
func GetConfig() (conf Config, err error) {
	var configDir string
	configDir, err = GetConfigDir()

	if err == nil {
		configFilepath := filepath.Join(configDir, configFilename)

		var bytes []byte
		if bytes, err = os.ReadFile(configFilepath); err == nil {
			if err = json.Unmarshal(bytes, &conf); err == nil {
				// fallback values
				if conf.TransmissionRPCPort <= 0 {
					conf.TransmissionRPCPort = consts.DefaultTransmissionRPCPort
				}
				if conf.MonitorInterval <= 0 {
					conf.MonitorInterval = consts.DefaultMonitorIntervalSeconds
				}

				return conf, nil
			}
		}
	}

	return conf, fmt.Errorf("failed to load config: %s", err)
}
