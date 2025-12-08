package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/meinside/telegram-remotecontrol-bot/cfg"
	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

func printUsage() {
	fmt.Printf(`* Usage:

	$ %[1]s [strings to broadcast]
`, os.Args[0])
}

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		var cliPort int

		// read port number from config file
		if config, err := cfg.GetConfig(context.TODO()); err == nil {
			cliPort = config.CLIPort
			if cliPort <= 0 {
				cliPort = consts.DefaultCLIPortNumber
			}
		} else {
			fmt.Printf("Failed to load config, using default port number: %d (%s)\n", consts.DefaultCLIPortNumber, err)

			cliPort = consts.DefaultCLIPortNumber
		}

		message := strings.Join(args, " ")

		if _, err := http.PostForm(
			fmt.Sprintf("http://localhost:%d%s", cliPort, consts.HTTPBroadcastPath),
			url.Values{
				consts.ParamMessage: {message},
			},
		); err != nil {
			fmt.Printf("*** %s\n", err)
		}
	} else {
		printUsage()
	}
}
