package helper

import (
	"fmt"
	"os/exec"
)

// for showing the list of transmission
func GetTransmissionList() string {
	if output, err := exec.Command("transmission-remote", "-l").CombinedOutput(); err == nil {
		return string(output)
	} else {
		return fmt.Sprintf("Failed to get transmission list - %s", string(output))
	}
}

// for adding a torrent to the list of transmission
func AddTransmissionTorrent(torrent string) string {
	if output, err := exec.Command("transmission-remote", "-a", torrent).CombinedOutput(); err == nil {
		return "Given torrent was successfully added to the list."
	} else {
		return fmt.Sprintf("Failed to add to transmission list - %s", string(output))
	}
}

// for canceling/removing a torrent from the list of transmission
func RemoveTransmissionTorrent(number string) string {
	if output, err := exec.Command("transmission-remote", "-t", number, "-r").CombinedOutput(); err == nil {
		return "Given torrent was successfully removed from the list."
	} else {
		return fmt.Sprintf("Failed to remove from transmission list - %s", string(output))
	}
}

// for removing a torrent and its local data from the list of transmission
func DeleteTransmissionTorrent(number string) string {
	if output, err := exec.Command("transmission-remote", "-t", number, "--remove-and-delete").CombinedOutput(); err == nil {
		return "Given torrent and its data were successfully deleted."
	} else {
		return fmt.Sprintf("Failed to delete from transmission list - %s", string(output))
	}
}
