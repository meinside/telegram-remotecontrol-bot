// telegram bot for controlling transmission remotely
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pkg/profile"

	bot "github.com/meinside/telegram-bot-go"

	"github.com/meinside/telegram-bot-remotecontrol/conf"
	"github.com/meinside/telegram-bot-remotecontrol/helper"
	"github.com/meinside/telegram-bot-remotecontrol/helper/services"
	"github.com/meinside/telegram-bot-remotecontrol/helper/services/transmission"
)

const (
	//DoProfiling = true
	DoProfiling = false
)

type Status int16

const (
	StatusWaiting                   Status = iota
	StatusWaitingServiceName        Status = iota
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

type KnownChatIds struct {
	ChatIds map[string]int
	sync.RWMutex
}

// variables
var apiToken string
var monitorInterval int
var isVerbose bool
var availableIds []string
var controllableServices []string
var pool SessionPool
var chatIds KnownChatIds
var queue chan string
var cliPort int
var launched time.Time
var db *helper.Database

// keyboards
var allKeyboards = [][]string{
	[]string{conf.CommandTransmissionList, conf.CommandTransmissionAdd, conf.CommandTransmissionRemove, conf.CommandTransmissionDelete},
	[]string{conf.CommandServiceStart, conf.CommandServiceStop},
	[]string{conf.CommandStatus, conf.CommandLogs, conf.CommandHelp},
}
var cancelKeyboard = [][]string{
	[]string{conf.CommandCancel},
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
	if config, err := helper.GetConfig(); err == nil {
		apiToken = config.ApiToken
		availableIds = config.AvailableIds
		controllableServices = config.ControllableServices
		monitorInterval = config.MonitorInterval
		if monitorInterval <= 0 {
			monitorInterval = conf.DefaultMonitorIntervalSeconds
		}
		isVerbose = config.IsVerbose

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
		chatIds = KnownChatIds{
			ChatIds: make(map[string]int),
		}
		queue = make(chan string, conf.QueueSize)

		// open database
		db = helper.OpenDb()
	} else {
		panic(err)
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

// check if given service is controllable
func isControllableService(service string) bool {
	for _, v := range controllableServices {
		if v == service {
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

/trlist : show torrent list
/tradd : add torrent with url or magnet
/trremove : remove torrent from list
/trdelete : remove torrent and delete data

*For Systemctl*

/servicestart _SERVICE_ : start a service
/servicestop _SERVICE_ : stop a service

*Others*

/status : show this bot's status
/help : show this help message
`
}

// get recent logs
func getLogs() string {
	var lines []string

	logs := db.GetLogs(conf.NumRecentLogs)

	if len(logs) <= 0 {
		return conf.MessageNoLogs
	} else {
		for _, log := range logs {
			lines = append(lines, fmt.Sprintf("%s %s: %s", log.Time.Format("2006-01-02 15:04:05"), log.Type, log.Message))
		}
		return strings.Join(lines, "\n")
	}
}

// for showing current status of this bot
func getStatus() string {
	return fmt.Sprintf("Uptime: %s\nMemory Usage: %s", helper.GetUptime(launched), helper.GetMemoryUsage())
}

// parse service command
func parseServiceCommand(txt string) (message string, keyboards [][]string) {
	message = conf.MessageNoControllableServices
	keyboards = nil

	for _, cmd := range []string{conf.CommandServiceStart, conf.CommandServiceStop} {
		if strings.HasPrefix(txt, cmd) {
			service := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

			if isControllableService(service) {
				if strings.HasPrefix(txt, conf.CommandServiceStart) { // start service
					if output, ok := services.Start(service); ok {
						message = fmt.Sprintf("Started service: *%s*", service)
					} else {
						message = output
					}
				} else if strings.HasPrefix(txt, conf.CommandServiceStop) { // stop service
					if output, ok := services.Stop(service); ok {
						message = fmt.Sprintf("Stopped service: *%s*", service)
					} else {
						message = output
					}
				}
			} else {
				message = conf.MessageControllableServices

				keys := []string{}
				for _, v := range controllableServices {
					keys = append(keys, fmt.Sprintf("%s %s", cmd, v))
				}

				keyboards = [][]string{
					keys,
					[]string{conf.CommandCancel},
				}
			}
		}
		continue
	}

	return message, keyboards
}

// process incoming update from Telegram
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

		// log error to db
		db.LogError(fmt.Sprintf("not allowed id: %s", userId))

		return false
	}

	// save chat id
	chatIds.Lock()
	chatIds.ChatIds[userId] = update.Message.Chat.Id
	chatIds.Unlock()

	// process result
	result := false

	pool.Lock()
	if session, exists := pool.Sessions[userId]; exists {
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
			// start
			case strings.HasPrefix(txt, conf.CommandStart):
				message = conf.MessageDefault
			// systemctl
			case strings.HasPrefix(txt, conf.CommandServiceStart) || strings.HasPrefix(txt, conf.CommandServiceStop):
				if len(controllableServices) > 0 {
					pool.Sessions[userId] = Session{
						UserId:        userId,
						CurrentStatus: StatusWaitingServiceName,
					}

					var keyboards [][]string
					message, keyboards = parseServiceCommand(txt)

					if keyboards != nil {
						options["reply_markup"] = bot.ReplyKeyboardMarkup{
							Keyboard:       keyboards,
							ResizeKeyboard: true,
						}
					}
				} else {
					message = conf.MessageNoControllableServices
				}
			// transmission
			case strings.HasPrefix(txt, conf.CommandTransmissionList):
				message = transmission.GetList()
			case strings.HasPrefix(txt, conf.CommandTransmissionAdd):
				message = conf.MessageTransmissionUpload
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionUpload,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			case strings.HasPrefix(txt, conf.CommandTransmissionRemove):
				message = conf.MessageTransmissionRemove
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionRemove,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			case strings.HasPrefix(txt, conf.CommandTransmissionDelete):
				message = conf.MessageTransmissionDelete
				pool.Sessions[userId] = Session{
					UserId:        userId,
					CurrentStatus: StatusWaitingTransmissionDelete,
				}
				options["reply_markup"] = bot.ReplyKeyboardMarkup{
					Keyboard:       cancelKeyboard,
					ResizeKeyboard: true,
				}
			case strings.HasPrefix(txt, conf.CommandStatus):
				message = getStatus()
			case strings.HasPrefix(txt, conf.CommandLogs):
				message = getLogs()
			case strings.HasPrefix(txt, conf.CommandHelp):
				message = getHelp()
			// fallback
			default:
				message = fmt.Sprintf("*%s*: %s", txt, conf.MessageUnknownCommand)
			}
		case StatusWaitingServiceName:
			switch {
			// systemctl
			case strings.HasPrefix(txt, conf.CommandServiceStart) || strings.HasPrefix(txt, conf.CommandServiceStop):
				if len(controllableServices) > 0 {
					pool.Sessions[userId] = Session{
						UserId:        userId,
						CurrentStatus: StatusWaiting,
					}

					var keyboards [][]string
					message, keyboards = parseServiceCommand(txt)

					if keyboards != nil {
						options["reply_markup"] = bot.ReplyKeyboardMarkup{
							Keyboard:       keyboards,
							ResizeKeyboard: true,
						}
					}
				} else {
					message = conf.MessageNoControllableServices
				}
			// cancel
			default:
				message = conf.MessageCanceled
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
			}
		case StatusWaitingTransmissionUpload:
			switch {
			case strings.HasPrefix(txt, conf.CommandCancel):
				message = conf.MessageCanceled
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
			case strings.HasPrefix(txt, conf.CommandCancel):
				message = conf.MessageCanceled
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
			case strings.HasPrefix(txt, conf.CommandCancel):
				message = conf.MessageCanceled
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

			// log error to db
			db.LogError(*sent.Description)
		}
	} else {
		log.Printf("*** Session does not exist for id: %s\n", userId)

		// log error to db
		db.LogError(fmt.Sprintf("no session for id: %s", userId))
	}
	pool.Unlock()

	return result
}

// for processing incoming request through HTTP
var httpHandler = func(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(r.FormValue(conf.ParamMessage))

	if len(message) > 0 {
		if isVerbose {
			log.Printf("Received message from CLI: %s\n", message)
		}

		queue <- message
	}
}

func main() {
	db.Log("Starting server...")

	client := bot.NewClient(apiToken)
	client.Verbose = isVerbose

	// get info about this bot
	if me := client.GetMe(); me.Ok {
		log.Printf("Launching bot: @%s (%s)\n", *me.Result.Username, *me.Result.FirstName)

		// delete webhook (getting updates will not work when wehbook is set up)
		if unhooked := client.DeleteWebhook(); unhooked.Ok {
			// wait for CLI message channel
			go func() {
				for {
					select {
					case message := <-queue:
						// broadcast to all connected chat ids
						chatIds.RLock()
						for _, chatId := range chatIds.ChatIds {
							if sent := client.SendMessage(chatId, &message, map[string]interface{}{}); !sent.Ok {
								log.Printf("*** Failed to broadcast to chat id %d: %s\n", chatId, *sent.Description)

								// log error to db
								db.LogError(*sent.Description)
							}
						}
						chatIds.RUnlock()
					}
				}
			}()

			// start web server for CLI
			go func() {
				if cliPort <= 0 {
					cliPort = conf.DefaultCliPortNumber
				}
				log.Printf("Starting local web server for CLI on port: %d\n", cliPort)

				http.HandleFunc(conf.HttpBroadcastPath, httpHandler)
				if err := http.ListenAndServe(fmt.Sprintf(":%d", cliPort), nil); err != nil {
					panic(err)
				}
			}()

			// wait for new updates
			client.StartMonitoringUpdates(0, monitorInterval, func(b *bot.Bot, update bot.Update, err error) {
				if err == nil {
					if update.Message != nil {
						processUpdate(b, update)
					}
				} else {
					log.Printf("*** Error while receiving update (%s)\n", err)
				}
			})
		} else {
			panic("Failed to delete webhook")
		}
	} else {
		panic("Failed to get info of the bot")
	}
}
