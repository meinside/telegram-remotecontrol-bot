// telegram bot for controlling transmission remotely
package main

import (
	"context"
	"time"

	"github.com/meinside/telegram-remotecontrol-bot/cfg"
)

func main() {
	launchedAt := time.Now()

	ctx := context.Background()

	// read config file,
	if config, err := cfg.GetConfig(ctx); err == nil {
		// and run the bot with it
		runBot(ctx, config, launchedAt)
	} else {
		panic(err)
	}
}
