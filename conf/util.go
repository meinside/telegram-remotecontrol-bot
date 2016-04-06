package conf

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"path/filepath"
	"runtime"
)

const (
	ConfigFilename = "../config.json"
)

// struct for config file
type Config struct {
	ApiToken             string   `json:"api_token"`
	AvailableIds         []string `json:"available_ids"`
	ControllableServices []string `json:"controllable_services"`
	MonitorInterval      int      `json:"monitor_interval"`
	CliPort              int      `json:"cli_port"`
	IsVerbose            bool     `json:"is_verbose"`
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
