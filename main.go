// telegram bot for controlling transmission remotely
package main

import (
	"time"

	"github.com/meinside/telegram-remotecontrol-bot/cfg"
)

func main() {
	launchedAt := time.Now()

	// read config file,
	if config, err := cfg.GetConfig(); err == nil {
		// and run the bot with it
		runBot(config, launchedAt)
	} else {
		panic(err)
	}
}
