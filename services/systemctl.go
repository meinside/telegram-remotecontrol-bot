package services

import (
	"fmt"
	"os/exec"
)

// systemctl start
func Start(service string) (message string, success bool) {
	if output, err := exec.Command("sudo", "systemctl", "start", service).CombinedOutput(); err == nil {
		return string(output), true
	} else {
		return fmt.Sprintf("Failed to start service: %s", service), false
	}
}

// systemctl stop
func Stop(service string) (message string, success bool) {
	if output, err := exec.Command("sudo", "systemctl", "stop", service).CombinedOutput(); err == nil {
		return string(output), true
	} else {
		return fmt.Sprintf("Failed to start service: %s", service), false
	}
}
