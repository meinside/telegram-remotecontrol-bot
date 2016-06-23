package services

import (
	"fmt"
	"os/exec"
	"strings"
)

// systemctl status (is-active)
func Status(services []string) (message string, success bool) {
	args := []string{"systemctl", "is-active"}
	args = append(args, services...)

	output, _ := exec.Command("sudo", args...).CombinedOutput()
	lines := []string{}
	for i, status := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		lines = append(lines, fmt.Sprintf("%s: *%s*", services[i], status))
	}

	return strings.Join(lines, "\n"), true
}

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
