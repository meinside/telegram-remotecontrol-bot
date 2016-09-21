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
	fmt.Printf(`* Usage:

	$ %[1]s [strings to broadcast]
`, os.Args[0])
}

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		// read variables from config file
		if config, err := helper.GetConfig(); err == nil {
			cliPort := config.CliPort
			if cliPort <= 0 {
				cliPort = conf.DefaultCliPortNumber
			}

			// XXX - remove markdown characters from given message
			message := helper.RemoveMarkdownChars(strings.Join(args, " "), " ")

			if _, err := http.PostForm(fmt.Sprintf("http://localhost:%d%s", cliPort, conf.HttpBroadcastPath), url.Values{
				conf.ParamMessage: {message},
			}); err != nil {
				fmt.Println(fmt.Errorf("*** %s", err))
			}
		} else {
			fmt.Println(fmt.Errorf("*** %s", err))
		}
	} else {
		printUsage()
	}
}
