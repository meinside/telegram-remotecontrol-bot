package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/meinside/telegram-bot-remotecontrol/conf"
	"github.com/meinside/telegram-bot-remotecontrol/helper"
)

func printUsage() {
	fmt.Printf(`* usage:

	$ %[1]s [strings to broadcast]
`, os.Args[0])
}

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		var cliPort int

		// read port number from config file
		if config, err := helper.GetConfig(); err == nil {
			cliPort = config.CliPort
			if cliPort <= 0 {
				cliPort = conf.DefaultCLIPortNumber
			}
		} else {
			fmt.Printf("failed to load config, using default port number: %d (%s)\n", conf.DefaultCLIPortNumber, err)

			cliPort = conf.DefaultCLIPortNumber
		}

		message := strings.Join(args, " ")

		if _, err := http.PostForm(fmt.Sprintf("http://localhost:%d%s", cliPort, conf.HTTPBroadcastPath), url.Values{
			conf.ParamMessage: {message},
		}); err != nil {
			fmt.Printf("*** %s\n", err)
		}
	} else {
		printUsage()
	}
}
