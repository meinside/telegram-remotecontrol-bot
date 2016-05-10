// telegram bot for controlling transmission remotely
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
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

	GithubPageUrl = "https://github.com/meinside/telegram-bot-remotecontrol"
)

type Status int16

const (
	StatusWaiting                   Status = iota
	StatusWaitingTransmissionUpload Status = iota
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
var controllableServices []string
var pool SessionPool
var queue chan string
var cliPort int
var launched time.Time
var db *helper.Database

// keyboards
var allKeyboards = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(conf.CommandTransmissionList, conf.CommandTransmissionAdd, conf.CommandTransmissionRemove, conf.CommandTransmissionDelete),
	bot.NewKeyboardButtons(conf.CommandServiceStart, conf.CommandServiceStop),
	bot.NewKeyboardButtons(conf.CommandStatus, conf.CommandLogs, conf.CommandHelp),
}
var cancelKeyboard = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(conf.CommandCancel),
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

/servicestart : start a service
/servicestop  : stop a service

*Others*

/status : show this bot's status
/logs : show latest logs of this bot
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

// parse service command and start/stop given service
func parseServiceCommand(txt string) (message string, keyboards [][]bot.InlineKeyboardButton) {
	message = conf.MessageNoControllableServices

	for _, cmd := range []string{conf.CommandServiceStart, conf.CommandServiceStop} {
		if strings.HasPrefix(txt, cmd) {
			service := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

			if isControllableService(service) {
				if strings.HasPrefix(txt, conf.CommandServiceStart) { // start service
					if output, ok := services.Start(service); ok {
						message = fmt.Sprintf("Started service: %s", service)
					} else {
						message = output
					}
				} else if strings.HasPrefix(txt, conf.CommandServiceStop) { // stop service
					if output, ok := services.Stop(service); ok {
						message = fmt.Sprintf("Stopped service: %s", service)
					} else {
						message = output
					}
				}
			} else {
				if strings.HasPrefix(txt, conf.CommandServiceStart) { // start service
					message = conf.MessageServiceToStart
				} else if strings.HasPrefix(txt, conf.CommandServiceStop) { // stop service
					message = conf.MessageServiceToStop
				}

				keys := map[string]string{}
				for _, v := range controllableServices {
					keys[v] = fmt.Sprintf("%s %s", cmd, v)
				}

				keyboards = [][]bot.InlineKeyboardButton{
					bot.NewInlineKeyboardButtonsWithCallbackData(keys),
				}
			}
		}
		continue
	}

	return message, keyboards
}

// parse transmission command
func parseTransmissionCommand(txt string) (message string, keyboards [][]bot.InlineKeyboardButton) {
	if torrents, _ := transmission.GetTorrents(); len(torrents) > 0 {
		for _, cmd := range []string{conf.CommandTransmissionRemove, conf.CommandTransmissionDelete} {
			if strings.HasPrefix(txt, cmd) {
				param := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

				if _, err := strconv.Atoi(param); err == nil { // if torrent id number is given,
					if strings.HasPrefix(txt, conf.CommandTransmissionRemove) { // remove torrent
						message = transmission.RemoveTorrent(param)
					} else if strings.HasPrefix(txt, conf.CommandTransmissionDelete) { // delete service
						message = transmission.DeleteTorrent(param)
					}
				} else {
					if strings.HasPrefix(txt, conf.CommandTransmissionRemove) { // remove torrent
						message = conf.MessageTransmissionRemove
					} else if strings.HasPrefix(txt, conf.CommandTransmissionDelete) { // delete torrent
						message = conf.MessageTransmissionDelete
					}

					// inline keyboards
					keys := map[string]string{}
					for _, t := range torrents {
						keys[fmt.Sprintf("%d. %s", t.Id, t.Name)] = fmt.Sprintf("%s %d", cmd, t.Id)
					}
					keyboards = [][]bot.InlineKeyboardButton{
						bot.NewInlineKeyboardButtonsWithCallbackData(keys),
					}
				}
			}
			continue
		}
	} else {
		message = conf.MessageTransmissionNoTorrents
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
	db.SaveChat(update.Message.Chat.Id, userId)

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
					var keyboards [][]bot.InlineKeyboardButton
					message, keyboards = parseServiceCommand(txt)

					if keyboards != nil {
						options["reply_markup"] = bot.InlineKeyboardMarkup{
							InlineKeyboard: keyboards,
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
			case strings.HasPrefix(txt, conf.CommandTransmissionRemove) || strings.HasPrefix(txt, conf.CommandTransmissionDelete):
				var keyboards [][]bot.InlineKeyboardButton
				message, keyboards = parseTransmissionCommand(txt)

				if keyboards != nil {
					options["reply_markup"] = bot.InlineKeyboardMarkup{
						InlineKeyboard: keyboards,
					}
				}
			case strings.HasPrefix(txt, conf.CommandStatus):
				message = getStatus()
			case strings.HasPrefix(txt, conf.CommandLogs):
				message = getLogs()
			case strings.HasPrefix(txt, conf.CommandHelp):
				message = getHelp()
				options["reply_markup"] = bot.InlineKeyboardMarkup{ // inline keyboard for link to github page
					InlineKeyboard: [][]bot.InlineKeyboardButton{
						bot.NewInlineKeyboardButtonsWithUrl(map[string]string{
							"GitHub": GithubPageUrl,
						}),
					},
				}
			// fallback
			default:
				message = fmt.Sprintf("*%s*: %s", txt, conf.MessageUnknownCommand)
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

// process incoming callback query
func processCallbackQuery(b *bot.Bot, update bot.Update) bool {
	query := *update.CallbackQuery
	txt := *query.Data

	// process result
	result := false

	var message string = ""
	if strings.HasPrefix(txt, conf.CommandServiceStart) || strings.HasPrefix(txt, conf.CommandServiceStop) { // service
		message, _ = parseServiceCommand(txt)
	} else if strings.HasPrefix(txt, conf.CommandTransmissionRemove) || strings.HasPrefix(txt, conf.CommandTransmissionDelete) { // transmission
		message, _ = parseTransmissionCommand(txt)
	} else {
		log.Printf("*** Unprocessable callback query: %s\n", txt)

		db.LogError(fmt.Sprintf("unprocessable callback query: %s", txt))
	}

	if len(message) > 0 {
		// answer callback query
		if apiResult := b.AnswerCallbackQuery(query.Id, map[string]interface{}{"text": message}); apiResult.Ok {
			// edit message and remove inline keyboards
			options := map[string]interface{}{
				"chat_id":    query.Message.Chat.Id,
				"message_id": query.Message.MessageId,
			}
			if apiResult := b.EditMessageText(&message, options); apiResult.Ok {
				result = true
			} else {
				log.Printf("*** Failed to edit message text: %s\n", *apiResult.Description)

				db.LogError(fmt.Sprintf("failed to edit message text: %s", *apiResult.Description))
			}
		} else {
			log.Printf("*** Failed to answer callback query: %+v\n", query)

			db.LogError(fmt.Sprintf("failed to answer callback query: %+v", query))
		}
	}

	return result
}

// broadcast a messge to given chats
func broadcast(client *bot.Bot, chats []helper.Chat, message string) {
	for _, chat := range chats {
		if isAvailableId(chat.UserId) {
			if sent := client.SendMessage(
				chat.ChatId,
				&message,
				map[string]interface{}{
					"parse_mode": bot.ParseModeMarkdown,
				}); !sent.Ok {
				log.Printf("*** Failed to broadcast to chat id %d: %s\n", chat.ChatId, *sent.Description)

				// log error to db
				db.LogError(*sent.Description)
			}
		} else {
			log.Printf("*** Id not allowed for broadcasting: %s\n", chat.UserId)

			// log error to db
			db.LogError(fmt.Sprintf("not allowed id for broadcasting: %s", chat.UserId))
		}
	}
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
	db.Log("starting server...")

	// catch SIGINT and SIGTERM and terminate gracefully
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		db.Log("stopping server...")
		os.Exit(1)
	}()

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
						// broadcast message from CLI
						broadcast(client, db.GetChats(), message)
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
					if update.HasMessage() {
						// process message
						processUpdate(b, update)
					} else if update.HasCallbackQuery() {
						// process callback query
						processCallbackQuery(b, update)
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
