// telegram bot for controlling transmission remotely
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/profile"

	bot "github.com/meinside/telegram-bot-go"

	"./services/transmission"
)

const (
	ConfigFilename = "config.json"

	//DoProfiling = true
	DoProfiling = false
)

// struct for config file
type Config struct {
	ApiToken             string   `json:"api_token"`
	AvailableIds         []string `json:"available_ids"`
	ControllableServices []string `json:"controllable_services"`
	MonitorInterval      int      `json:"monitor_interval"`
	IsVerbose            bool     `json:"is_verbose"`
}

const (
	DefaultMonitorIntervalSeconds = 3
)

const (
	// commands
	CommandStart  = "/start"
	CommandHelp   = "/help"
	CommandStatus = "/status"
	CommandCancel = "/cancel"

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

// keyboards
var allKeyboards = [][]string{
	[]string{CommandTransmissionList, CommandTransmissionAdd, CommandTransmissionRemove, CommandTransmissionDelete},
	[]string{CommandStatus, CommandHelp},
}
var cancelKeyboard = [][]string{
	[]string{CommandCancel},
}

// initialization
func init() {
	launched = time.Now()

	// for profiling
	if DoProfiling {
		defer profile.Start(
			profile.BlockProfile,
			profile.CPUProfile,
			profile.MemProfile,
		).Stop()
	}

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

*For Transmission*

/list   : show torrent list
/add    : add torrent with url or magnet
/remove : remove torrent from list
/delete : remove torrent and delete data

*Others*

/status   : show this bot's status
/help     : show this help message
`
}

// for showing current status of this bot
func getStatus() string {
	return fmt.Sprintf("Uptime: %s\nMemory Usage: %s", getUptime(launched), getMemoryUsage())
}

// for processing incoming update from Telegram
func processUpdate(b *bot.Bot, update bot.Update) bool {
	// check username
	var userId string
	if update.Message.From.Username == nil {
		log.Printf("*** Not allowed (no user name): %s\n", *update.Message.From.FirstName)
		return false
	}
	userId = *update.Message.From.Username
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
		if update.Message.HasText() {
			txt = *update.Message.Text
		} else {
			txt = ""
		}

		var message string
		var options map[string]interface{} = map[string]interface{}{
			"reply_markup": bot.ReplyKeyboardMarkup{
				Keyboard:       allKeyboards,
				ResizeKeyboard: true,
			},
			"parse_mode": bot.ParseModeMarkdown,
		}

		switch session.CurrentStatus {
		case StatusWaiting:
			switch {
			case strings.HasPrefix(txt, CommandStart):
				message = DefaultMessage
			case strings.HasPrefix(txt, CommandHelp):
				message = getHelp()
			case strings.HasPrefix(txt, CommandStatus):
				message = getStatus()
			case strings.HasPrefix(txt, CommandTransmissionList):
				message = transmission.GetList()
			case strings.HasPrefix(txt, CommandTransmissionAdd):
				message = MessageTransmissionUpload
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionUpload,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			case strings.HasPrefix(txt, CommandTransmissionRemove):
				message = MessageTransmissionRemove
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionRemove,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			case strings.HasPrefix(txt, CommandTransmissionDelete):
				message = MessageTransmissionDelete
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionDelete,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			default:
				message = fmt.Sprintf("*%s*: %s", txt, MessageUnknownCommand)
			}
		case StatusWaitingTransmissionUpload:
			switch {
			case strings.HasPrefix(txt, CommandCancel):
				message = MessageCanceled
			default:
				var torrent string
				if update.Message.Document != nil {
					fileResult := b.GetFile(update.Message.Document.FileId)
					torrent = b.GetFileUrl(*fileResult.Result)
				} else {
					torrent = txt
				}

				message = transmission.AddTorrent(torrent)
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
				message = transmission.RemoveTorrent(txt)
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
				message = transmission.DeleteTorrent(txt)
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		}

		// send message
		if sent := b.SendMessage(update.Message.Chat.Id, &message, options); sent.Ok {
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

// get uptime of this bot in seconds
func getUptime(launched time.Time) (uptime string) {
	now := time.Now()
	gap := now.Sub(launched)

	uptimeSeconds := int(gap.Seconds())
	numDays := uptimeSeconds / (60 * 60 * 24)
	numHours := (uptimeSeconds % (60 * 60 * 24)) / (60 * 60)

	return fmt.Sprintf("*%d* day(s) *%d* hour(s)", numDays, numHours)
}

// get memory usage
func getMemoryUsage() (usage string) {
	m := new(runtime.MemStats)
	runtime.ReadMemStats(m)

	return fmt.Sprintf("Sys: *%.1f MB*, Heap: *%.1f MB*", float32(m.Sys)/1024/1024, float32(m.HeapAlloc)/1024/1024)
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
						processUpdate(b, update)
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
