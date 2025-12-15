// cmd/telegram-bot-broadcast/main.go

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jessevdk/go-flags"

	"github.com/meinside/telegram-remotecontrol-bot/cfg"
	"github.com/meinside/telegram-remotecontrol-bot/consts"
)

const (
	usageFormat = `-m "MESSAGE_TO_BROADCAST"
  %[1]s "MESSAGE_TO_BROADCAST"
  echo "something" | %[1]s`
)

// struct for parameters
type params struct {
	Message *string `short:"m" long:"message" description:"Message to broadcast"`
}

func main() {
	// parse params,
	var p params
	parser := flags.NewParser(
		&p,
		flags.HelpFlag|flags.PassDoubleDash,
	)
	parser.Usage = fmt.Sprintf(usageFormat, filepath.Base(os.Args[0]))
	if _, err := parser.Parse(); err == nil {
		// get message from params,
		var message string
		if p.Message != nil {
			message = *p.Message
		} else {
			args := os.Args[1:]
			if len(args) > 0 {
				message = strings.Join(args, " ")
			}
		}

		// read message from standard input, if any
		var stdin []byte
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			stdin, _ = io.ReadAll(os.Stdin)
		}
		if len(stdin) > 0 {
			if len(message) <= 0 {
				message = string(stdin)
			} else {
				// merge message from stdin and parameter
				message = string(stdin) + "\n\n" + message
			}
		}

		// check message
		if len(message) <= 0 {
			printHelpAndExit(1, parser)
		}

		// read port number from config file,
		var cliPort int
		if config, err := cfg.GetConfig(context.TODO()); err == nil {
			cliPort = config.CLIPort
			if cliPort <= 0 {
				cliPort = consts.DefaultCLIPortNumber
			}
		} else {
			fmt.Printf("Failed to load config, using default port number: %d (%s)\n", consts.DefaultCLIPortNumber, err)

			cliPort = consts.DefaultCLIPortNumber
		}

		// send message to local API,
		if _, err := http.PostForm(
			fmt.Sprintf("http://localhost:%d%s", cliPort, consts.HTTPBroadcastPath),
			url.Values{
				consts.ParamMessage: {message},
			},
		); err != nil {
			fmt.Printf("* Broadcast failed: %s\n", err)
		}
	} else {
		if e, ok := err.(*flags.Error); ok {
			if e.Type != flags.ErrHelp {
				fmt.Printf("Input error: %s\n", e.Error())
			}

			printHelpAndExit(1, parser)
		}

		printErrorAndExit(1, "Failed to parse flags: %s\n", err)
	}
}

// print help and exit
func printHelpAndExit(
	exit int,
	parser *flags.Parser,
) {
	parser.WriteHelp(os.Stderr)
	os.Exit(exit)
}

// print error and exit
func printErrorAndExit(
	exit int,
	format string,
	a ...any,
) {
	fmt.Printf(format, a...)
	os.Exit(exit)
}
