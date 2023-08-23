package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meinside/infisical-go"
	"github.com/meinside/infisical-go/helper"

	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

// constants for config
const (
	AppName        = "telegram-remotecontrol-bot"
	configFilename = "config.json"
)

// Config struct for config file
type Config struct {
	AvailableIDs            []string `json:"available_ids"`
	ControllableServices    []string `json:"controllable_services,omitempty"`
	MountPoints             []string `json:"mount_points,omitempty"`
	MonitorInterval         int      `json:"monitor_interval"`
	TransmissionRPCPort     int      `json:"transmission_rpc_port,omitempty"`
	TransmissionRPCUsername string   `json:"transmission_rpc_username,omitempty"`
	TransmissionRPCPasswd   string   `json:"transmission_rpc_passwd,omitempty"`
	CLIPort                 int      `json:"cli_port"`
	IsVerbose               bool     `json:"is_verbose"`

	APIToken string `json:"api_token,omitempty"`

	// or Infisical settings
	Infisical *struct {
		// NOTE: When the workspace's E2EE setting is enabled, APIKey is essential for decryption
		E2EE   bool    `json:"e2ee,omitempty"`
		APIKey *string `json:"api_key,omitempty"`

		WorkspaceID string               `json:"workspace_id"`
		Token       string               `json:"token"`
		Environment string               `json:"environment"`
		SecretType  infisical.SecretType `json:"secret_type"`

		APITokenKeyPath string `json:"api_token_key_path"`
	} `json:"infisical,omitempty"`
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
				if conf.APIToken == "" && conf.Infisical != nil {
					var apiToken string

					// read access token from infisical
					if conf.Infisical.E2EE && conf.Infisical.APIKey != nil {
						apiToken, err = helper.E2EEValue(
							*conf.Infisical.APIKey,
							conf.Infisical.WorkspaceID,
							conf.Infisical.Token,
							conf.Infisical.Environment,
							conf.Infisical.SecretType,
							conf.Infisical.APITokenKeyPath,
						)
					} else {
						apiToken, err = helper.Value(
							conf.Infisical.WorkspaceID,
							conf.Infisical.Token,
							conf.Infisical.Environment,
							conf.Infisical.SecretType,
							conf.Infisical.APITokenKeyPath,
						)
					}
					conf.APIToken = apiToken
				}

				// fallback values
				if conf.TransmissionRPCPort <= 0 {
					conf.TransmissionRPCPort = consts.DefaultTransmissionRPCPort
				}
				if conf.MonitorInterval <= 0 {
					conf.MonitorInterval = consts.DefaultMonitorIntervalSeconds
				}

				return conf, err
			}
		}
	}

	return conf, fmt.Errorf("failed to load config: %s", err)
}
