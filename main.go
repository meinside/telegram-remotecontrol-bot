// telegram bot for controlling transmission remotely
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	bot "github.com/meinside/telegram-bot-go"

	"github.com/meinside/telegram-bot-transmission/helper"
)

const (
	ConfigFilename = "config.json"

	BotVersion = "0.0.4.20160226"
)

// struct for config file
type Config struct {
	ApiToken        string   `json:"api_token"`
	AvailableIds    []string `json:"available_ids"`
	MonitorInterval int      `json:"monitor_interval"`
	IsVerbose       bool     `json:"is_verbose"`
}

const (
	DefaultMonitorIntervalSeconds = 3
)

const (
	// commands
	CommandStart   = "/start"
	CommandHelp    = "/help"
	CommandVersion = "/version"
	CommandStatus  = "/status"
	CommandCancel  = "/cancel"

	// commands for transmission
	CommandTransmissionList   = "/list"
	CommandTransmissionAdd    = "/add"
	CommandTransmissionRemove = "/remove"
	CommandTransmissionDelete = "/delete"
)

const (
	// messages
	DefaultMessage            = "Input your command:"
	MessageUnknownCommand     = "Unknown command."
	MessageTransmissionUpload = "Input magnet, url, or file of target torrent:"
	MessageTransmissionRemove = "Input the number of torrent to remove from the list:"
	MessageTransmissionDelete = "Input the number of torrent to delete from the list and local storage:"
	MessageCanceled           = "Canceled."
)

type Status int16

const (
	StatusWaiting                   Status = iota
	StatusWaitingTransmissionUpload Status = iota
	StatusWaitingTransmissionRemove Status = iota
	StatusWaitingTransmissionDelete Status = iota
)

type Session struct {
	UserId        string
	CurrentStatus Status
}

type SessionPool struct {
	Sessions map[string]Session
	sync.Mutex
}

// variables
var apiToken string
var monitorInterval int
var isVerbose bool
var availableIds []string
var pool SessionPool
var launched time.Time

func init() {
	launched = time.Now()

	// read variables from config file
	if file, err := ioutil.ReadFile(ConfigFilename); err == nil {
		var conf Config
		if err := json.Unmarshal(file, &conf); err == nil {
			apiToken = conf.ApiToken
			availableIds = conf.AvailableIds
			monitorInterval = conf.MonitorInterval
			if monitorInterval <= 0 {
				monitorInterval = DefaultMonitorIntervalSeconds
			}
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
/delete : remove torrent and delete data

* Others

/version  : show this bot's version
/status   : show this bot's status
/help     : show this help message
`
}

// for showing the version of this bot
func getVersion() string {
	return fmt.Sprintf("Bot version: %s", BotVersion)
}

// for showing current status of this bot
func getStatus() string {
	return fmt.Sprintf("Uptime: %s\nMemory Usage: %s", helper.GetUptime(launched), helper.GetMemoryUsage())
}

// for processing incoming webhook from Telegram
func processWebhook(b *bot.Bot, webhook bot.Update) bool {
	// check username
	var userId string
	if webhook.Message.From.Username == nil {
		log.Printf("*** Not allowed (no user name): %s\n", *webhook.Message.From.FirstName)
		return false
	}
	userId = *webhook.Message.From.Username
	if !isAvailableId(userId) {
		log.Printf("*** Id not allowed: %s\n", userId)
		return false
	}

	// process result
	result := false

	if session, exists := pool.Sessions[userId]; exists {
		pool.Lock()

		// text from message
		var txt string
		if webhook.Message.HasText() {
			txt = *webhook.Message.Text
		} else {
			txt = ""
		}

		var message string
		var options map[string]interface{} = map[string]interface{}{
			"reply_markup": bot.ReplyKeyboardMarkup{
				Keyboard: [][]string{
					[]string{CommandTransmissionList, CommandTransmissionAdd, CommandTransmissionRemove, CommandTransmissionDelete},
					[]string{CommandVersion, CommandStatus, CommandHelp},
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
			case strings.HasPrefix(txt, CommandStatus):
				message = getStatus()
			case strings.HasPrefix(txt, CommandTransmissionList):
				message = helper.GetTransmissionList()
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
			case strings.HasPrefix(txt, CommandTransmissionDelete):
				message = MessageTransmissionDelete
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionDelete,
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
					fileResult := b.GetFile(webhook.Message.Document.FileId)
					torrent = b.GetFileUrl(*fileResult.Result)
				} else {
					torrent = txt
				}

				message = helper.AddTransmissionTorrent(torrent)
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
				message = helper.RemoveTransmissionTorrent(txt)
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		case StatusWaitingTransmissionDelete:
			switch {
			case strings.HasPrefix(txt, CommandCancel):
				message = MessageCanceled
			default:
				message = helper.DeleteTransmissionTorrent(txt)
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		}

		// send message
		if sent := b.SendMessage(webhook.Message.Chat.Id, &message, options); sent.Ok {
			result = true
		} else {
			log.Printf("*** Failed to send message: %s\n", *sent.Description)
		}

		pool.Unlock()
	} else {
		log.Printf("*** Session does not exist for id: %s\n", userId)
	}

	return result
}

func main() {
	client := bot.NewClient(apiToken)
	client.Verbose = isVerbose

	// get info about this bot
	if me := client.GetMe(); me.Ok {
		log.Printf("Launching bot: @%s (%s)\n", *me.Result.Username, *me.Result.FirstName)

		// delete webhook (getting updates will not work when wehbook is set up)
		if unhooked := client.DeleteWebhook(); unhooked.Ok {
			// wait for new updates
			client.StartMonitoringUpdates(0, monitorInterval, func(b *bot.Bot, update bot.Update, err error) {
				if err == nil {
					if update.Message != nil {
						processWebhook(b, update)
					}
				} else {
					log.Printf("*** Error while receiving update (%s)\n", err.Error())
				}
			})
		} else {
			panic("Failed to delete webhook")
		}
	} else {
		panic("Failed to get info of the bot")
	}
}
