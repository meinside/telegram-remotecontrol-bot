// telegram bot for controlling transmission remotely
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"

	bot "github.com/meinside/telegram-bot-go"
)

const (
	ConfigFilename = "config.json"

	BotVersion = "0.0.1.20151129"
)

// struct for config file
type Config struct {
	ApiToken     string   `json:"api_token"`
	WebhookHost  string   `json:"webhook_host"`
	WebhookPort  int      `json:"webhook_port"`
	CertFilename string   `json:"cert_filename"`
	KeyFilename  string   `json:"key_filename"`
	AvailableIds []string `json:"available_ids"`
	IsVerbose    bool     `json:"is_verbose"`
}

const (
	// messages
	DefaultMessage            = "Input your command."
	MessageUnknownCommand     = "Unknown command."
	MessageTransmissionUpload = "Input magnet, url, or file of target torrent."
	MessageTransmissionRemove = "Input the number of torrent to remove from the list."
	MessageCanceled           = "Canceled."

	// commands
	CommandStart   = "/start"
	CommandHelp    = "/help"
	CommandVersion = "/version"
	CommandCancel  = "/cancel"

	// commands for transmission
	CommandTransmissionList   = "/list"
	CommandTransmissionAdd    = "/add"
	CommandTransmissionRemove = "/remove"
)

type Status int16

const (
	StatusWaiting                   Status = iota
	StatusWaitingTransmissionUpload Status = iota
	StatusWaitingTransmissionRemove Status = iota
)

type Session struct {
	UserId        string
	CurrentStatus Status
}

type SessionPool struct {
	Sessions map[string]Session
	Mutex    sync.Mutex
}

// variables
var apiToken, webhookHost, certFilename, keyFilename string
var webhookPort int
var isVerbose bool
var availableIds []string
var pool SessionPool

func init() {
	// read variables from config file
	if file, err := ioutil.ReadFile(ConfigFilename); err == nil {
		var conf Config
		if err := json.Unmarshal(file, &conf); err == nil {
			apiToken = conf.ApiToken
			webhookHost = conf.WebhookHost
			webhookPort = conf.WebhookPort
			certFilename = conf.CertFilename
			keyFilename = conf.KeyFilename
			availableIds = conf.AvailableIds
			isVerbose = conf.IsVerbose

			// initialize variables
			sessions := make(map[string]Session)
			for _, v := range availableIds {
				sessions[v] = Session{
					UserId:        v,
					CurrentStatus: StatusWaiting,
				}
			}
			pool = SessionPool{
				Sessions: sessions,
			}
		} else {
			panic(err.Error())
		}
	} else {
		panic(err.Error())
	}
}

// check if given Telegram id is available
func isAvailableId(id string) bool {
	for _, v := range availableIds {
		if v == id {
			return true
		}
	}
	return false
}

// for showing help message
func getHelp() string {
	return `
Following commands are supported:

* For Transmission

/list   : show torrent list
/add    : add torrent with url or magnet
/remove : remove torrent from list

* Others

/version  : show this bot's version
/help     : show this help message
`
}

// for showing the version of this bot
func getVersion() string {
	return fmt.Sprintf("Bot version: %s", BotVersion)
}

// for showing the list of transmission
func getTransmissionList() string {
	if output, err := exec.Command("transmission-remote", "-l").CombinedOutput(); err == nil {
		return string(output)
	} else {
		return fmt.Sprintf("Failed to get transmission list - %s", string(output))
	}
}

// for adding a torrent to the list of transmission
func addTransmissionTorrent(torrent string) string {
	if output, err := exec.Command("transmission-remote", "-a", torrent).CombinedOutput(); err == nil {
		return "Given torrent was successfully added to the list."
	} else {
		return fmt.Sprintf("Failed to add to transmission list - %s", string(output))
	}
}

// for canceling/removing a torrent from the list of transmission
func removeTransmissionTorrent(number string) string {
	if output, err := exec.Command("transmission-remote", "-t", number, "-r").CombinedOutput(); err == nil {
		return "Given torrent was successfully removed from the list."
	} else {
		return fmt.Sprintf("Failed to remove from transmission list - %s", string(output))
	}
}

// for processing incoming webhook from Telegram
func processWebhook(client *bot.Bot, webhook bot.Webhook) bool {
	// check username
	var userId string
	if webhook.Message.From.Username == nil {
		fmt.Printf("*** Not allowed (no user name): %s\n", *webhook.Message.From.FirstName)
		return false
	}
	userId = *webhook.Message.From.Username
	if !isAvailableId(userId) {
		fmt.Printf("*** Id not allowed: %s\n", userId)
		return false
	}

	// process result
	result := false

	if session, exists := pool.Sessions[userId]; exists {
		pool.Mutex.Lock()

		// text from message
		var txt string
		if webhook.Message.Text != nil {
			txt = *webhook.Message.Text
		} else {
			txt = ""
		}

		var message string
		var options map[string]interface{} = map[string]interface{}{
			"reply_markup": bot.ReplyKeyboardMarkup{
				Keyboard: [][]string{
					[]string{CommandTransmissionList, CommandTransmissionAdd, CommandTransmissionRemove},
					[]string{CommandVersion, CommandHelp},
				},
			},
		}

		switch session.CurrentStatus {
		case StatusWaiting:
			switch {
			case strings.HasPrefix(txt, CommandStart):
				message = DefaultMessage
			case strings.HasPrefix(txt, CommandHelp):
				message = getHelp()
			case strings.HasPrefix(txt, CommandVersion):
				message = getVersion()
			case strings.HasPrefix(txt, CommandTransmissionList):
				message = getTransmissionList()
			case strings.HasPrefix(txt, CommandTransmissionAdd):
				message = MessageTransmissionUpload
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionUpload,
				}
				options = map[string]interface{}{
					"reply_markup": bot.ReplyKeyboardMarkup{
						Keyboard: [][]string{
							[]string{CommandCancel},
						},
					},
				}
			case strings.HasPrefix(txt, CommandTransmissionRemove):
				message = MessageTransmissionRemove
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionRemove,
				}
				options = map[string]interface{}{
					"reply_markup": bot.ReplyKeyboardMarkup{
						Keyboard: [][]string{
							[]string{CommandCancel},
						},
					},
				}
			default:
				message = fmt.Sprintf("%s: %s", txt, MessageUnknownCommand)
			}
		case StatusWaitingTransmissionUpload:
			switch {
			case strings.HasPrefix(txt, CommandCancel):
				message = MessageCanceled
			default:
				var torrent string
				if webhook.Message.Document != nil {
					fileResult := client.GetFile(webhook.Message.Document.FileId)
					torrent = client.GetFileUrl(*fileResult.Result)
				} else {
					torrent = txt
				}

				message = addTransmissionTorrent(torrent)
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		case StatusWaitingTransmissionRemove:
			switch {
			case strings.HasPrefix(txt, CommandCancel):
				message = MessageCanceled
			default:
				message = removeTransmissionTorrent(txt)
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		}

		// send message
		if sent := client.SendMessage(webhook.Message.Chat.Id, &message, options); sent.Ok {
			result = true
		} else {
			fmt.Printf("*** Failed to send message: %s\n", *sent.Description)
		}

		pool.Mutex.Unlock()
	} else {
		fmt.Printf("*** Session does not exist for id: %s\n", userId)
	}

	return result
}

func main() {
	client := bot.NewClient(apiToken)
	client.Verbose = isVerbose

	// get info about this bot
	if me := client.GetMe(); me.Ok {
		fmt.Printf("Launching bot: @%s (%s)\n", *me.Result.Username, *me.Result.FirstName)

		// set webhook url
		if hooked := client.SetWebhook(webhookHost, webhookPort, certFilename); hooked.Ok {
			// on success, start webhook server
			client.StartWebhookServerAndWait(certFilename, keyFilename, func(webhook bot.Webhook, success bool, err error) {
				if success {
					processWebhook(client, webhook)
				} else {
					fmt.Printf("*** Error while receiving webhook (%s)\n", err.Error())
				}
			})
		} else {
			panic("Failed to set webhook")
		}
	} else {
		panic("Failed to get info of the bot")
	}
}
