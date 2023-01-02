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

	bot "github.com/meinside/telegram-bot-go"

	"github.com/meinside/telegram-bot-remotecontrol/conf"
	"github.com/meinside/telegram-bot-remotecontrol/helper"
	"github.com/meinside/telegram-bot-remotecontrol/helper/services/transmission"
)

const (
	githubPageURL = "https://github.com/meinside/telegram-bot-remotecontrol"
)

type status int16

// application statuses
const (
	StatusWaiting                   status = iota
	StatusWaitingTransmissionUpload status = iota
)

type session struct {
	UserID        string
	CurrentStatus status
}

type sessionPool struct {
	Sessions map[string]session
	sync.Mutex
}

// variables
var apiToken string
var monitorInterval int
var isVerbose bool
var availableIds []string
var controllableServices []string
var mountPoints []string
var rpcPort int
var rpcUsername, rpcPasswd string
var pool sessionPool
var queue chan string
var cliPort int
var launched time.Time
var db *helper.Database

// keyboards
var allKeyboards = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(conf.CommandTransmissionList, conf.CommandTransmissionAdd, conf.CommandTransmissionRemove, conf.CommandTransmissionDelete),
	bot.NewKeyboardButtons(conf.CommandServiceStatus, conf.CommandServiceStart, conf.CommandServiceStop),
	bot.NewKeyboardButtons(conf.CommandStatus, conf.CommandLogs, conf.CommandHelp),
}
var cancelKeyboard = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(conf.CommandCancel),
}

var _stdout = log.New(os.Stdout, "", log.LstdFlags)
var _stderr = log.New(os.Stderr, "", log.LstdFlags)

// initialization
func init() {
	launched = time.Now()

	// read variables from config file
	if config, err := helper.GetConfig(); err == nil {
		apiToken = config.APIToken
		availableIds = config.AvailableIds
		controllableServices = config.ControllableServices
		mountPoints = config.MountPoints
		rpcPort = config.TransmissionRPCPort
		if rpcPort <= 0 {
			rpcPort = conf.DefaultTransmissionRPCPort
		}
		rpcUsername = config.TransmissionRPCUsername
		rpcPasswd = config.TransmissionRPCPasswd
		monitorInterval = config.MonitorInterval
		if monitorInterval <= 0 {
			monitorInterval = conf.DefaultMonitorIntervalSeconds
		}
		isVerbose = config.IsVerbose

		// initialize variables
		sessions := make(map[string]session)
		for _, v := range availableIds {
			sessions[v] = session{
				UserID:        v,
				CurrentStatus: StatusWaiting,
			}
		}
		pool = sessionPool{
			Sessions: sessions,
		}
		queue = make(chan string, conf.QueueSize)

		// open database
		db = helper.OpenDB()
	} else {
		panic(err)
	}
}

// check if given Telegram id is available
func isAvailableID(id string) bool {
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
	return fmt.Sprintf(`
following commands are supported:

*for transmission*

%s : show torrent list
%s : add torrent with url or magnet
%s : remove torrent from list
%s : remove torrent and delete data

*for systemctl*

%s : show status of each service (systemctl is-active)
%s : start a service (systemctl start)
%s : stop a service (systemctl stop)

*others*

%s : show this bot's status
%s : show latest logs of this bot
%s : show this help message
`,
		conf.CommandTransmissionList,
		conf.CommandTransmissionAdd,
		conf.CommandTransmissionRemove,
		conf.CommandTransmissionDelete,
		conf.CommandServiceStatus,
		conf.CommandServiceStart,
		conf.CommandServiceStop,
		conf.CommandStatus,
		conf.CommandLogs,
		conf.CommandHelp,
	)
}

// get recent logs
func getLogs() string {
	var lines []string

	logs := db.GetLogs(conf.NumRecentLogs)

	if len(logs) <= 0 {
		return conf.MessageNoLogs
	}

	for _, log := range logs {
		lines = append(lines, fmt.Sprintf("%s %s: %s", log.Time.Format("2006-01-02 15:04:05"), log.Type, log.Message))
	}
	return strings.Join(lines, "\n")
}

// for showing current status of this bot
func getStatus() string {
	return fmt.Sprintf("app uptime: %s\napp memory usage: %s\nsystem disk usage:\n%s", helper.GetUptime(launched), helper.GetMemoryUsage(), helper.GetDiskUsage(mountPoints))
}

// parse service command and start/stop given service
func parseServiceCommand(txt string) (message string, keyboards [][]bot.InlineKeyboardButton) {
	message = conf.MessageNoControllableServices

	for _, cmd := range []string{conf.CommandServiceStart, conf.CommandServiceStop} {
		if strings.HasPrefix(txt, cmd) {
			service := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

			if isControllableService(service) {
				if strings.HasPrefix(txt, conf.CommandServiceStart) { // start service
					if output, err := helper.SystemctlStart(service); err == nil {
						message = fmt.Sprintf("started service: %s", service)
					} else {
						message = fmt.Sprintf("failed to start service: %s (%s)", service, err)

						db.LogError(fmt.Sprintf("service failed to start: %s", output))
					}
				} else if strings.HasPrefix(txt, conf.CommandServiceStop) { // stop service
					if output, err := helper.SystemctlStop(service); err == nil {
						message = fmt.Sprintf("stopped service: %s", service)
					} else {
						message = fmt.Sprintf("failed to stop service: %s (%s)", service, err)

						db.LogError(fmt.Sprintf("service failed to stop: %s", output))
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
				keyboards = bot.NewInlineKeyboardButtonsAsRowsWithCallbackData(keys)

				// add cancel button
				cancel := conf.CommandCancel
				keyboards = append(keyboards, []bot.InlineKeyboardButton{
					{
						Text:         conf.MessageCancel,
						CallbackData: &cancel,
					},
				})
			}
		}
		continue
	}

	return message, keyboards
}

// parse transmission command
func parseTransmissionCommand(txt string) (message string, keyboards [][]bot.InlineKeyboardButton) {
	if torrents, _ := transmission.GetTorrents(rpcPort, rpcUsername, rpcPasswd); len(torrents) > 0 {
		for _, cmd := range []string{conf.CommandTransmissionRemove, conf.CommandTransmissionDelete} {
			if strings.HasPrefix(txt, cmd) {
				param := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

				if _, err := strconv.Atoi(param); err == nil { // if torrent id number is given,
					if strings.HasPrefix(txt, conf.CommandTransmissionRemove) { // remove torrent
						message = transmission.RemoveTorrent(rpcPort, rpcUsername, rpcPasswd, param)
					} else if strings.HasPrefix(txt, conf.CommandTransmissionDelete) { // delete service
						message = transmission.DeleteTorrent(rpcPort, rpcUsername, rpcPasswd, param)
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
						keys[fmt.Sprintf("%d. %s", t.ID, t.Name)] = fmt.Sprintf("%s %d", cmd, t.ID)
					}
					keyboards = bot.NewInlineKeyboardButtonsAsRowsWithCallbackData(keys)

					// add cancel button
					cancel := conf.CommandCancel
					keyboards = append(keyboards, []bot.InlineKeyboardButton{
						{
							Text:         conf.MessageCancel,
							CallbackData: &cancel,
						},
					})
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
	var userID string
	if update.Message.From.Username == nil {
		_stderr.Printf("not allowed, or has no username: %s", update.Message.From.FirstName)
		return false
	}
	userID = *update.Message.From.Username
	if !isAvailableID(userID) {
		_stderr.Printf("id not allowed: %s", userID)

		// log error to db
		db.LogError(fmt.Sprintf("not allowed id: %s", userID))

		return false
	}

	// save chat id
	db.SaveChat(update.Message.Chat.ID, userID)

	// process result
	result := false

	pool.Lock()
	if s, exists := pool.Sessions[userID]; exists {
		// text from message
		var txt string
		if update.Message.HasText() {
			txt = *update.Message.Text
		} else {
			txt = ""
		}

		var message string
		var options = defaultOptions()

		switch s.CurrentStatus {
		case StatusWaiting:
			if update.Message.Document != nil { // if a file is received,
				fileResult := b.GetFile(update.Message.Document.FileID)
				fileURL := b.GetFileURL(*fileResult.Result)

				// XXX - only support: .torrent
				if strings.HasSuffix(fileURL, ".torrent") {
					message = transmission.AddTorrent(rpcPort, rpcUsername, rpcPasswd, fileURL)
				} else {
					message = conf.MessageUnprocessableFileFormat
				}
			} else {
				switch {
				// magnet url
				case strings.HasPrefix(txt, "magnet:"):
					message = transmission.AddTorrent(rpcPort, rpcUsername, rpcPasswd, txt)
				// /start
				case strings.HasPrefix(txt, conf.CommandStart):
					message = conf.MessageDefault
				// systemctl
				case strings.HasPrefix(txt, conf.CommandServiceStatus):
					statuses, _ := helper.SystemctlStatus(controllableServices)
					for service, status := range statuses {
						message += fmt.Sprintf("%s: *%s*\n", service, status)
					}
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
					message = transmission.GetList(rpcPort, rpcUsername, rpcPasswd)
				case strings.HasPrefix(txt, conf.CommandTransmissionAdd):
					arg := strings.TrimSpace(strings.Replace(txt, conf.CommandTransmissionAdd, "", 1))
					if strings.HasPrefix(arg, "magnet:") {
						message = transmission.AddTorrent(rpcPort, rpcUsername, rpcPasswd, arg)
					} else {
						message = conf.MessageTransmissionUpload
						pool.Sessions[userID] = session{
							UserID:        userID,
							CurrentStatus: StatusWaitingTransmissionUpload,
						}
						options["reply_markup"] = bot.ReplyKeyboardMarkup{
							Keyboard:       cancelKeyboard,
							ResizeKeyboard: true,
						}
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
							bot.NewInlineKeyboardButtonsWithURL(map[string]string{
								"GitHub": githubPageURL,
							}),
						},
					}
				// fallback
				default:
					cmd := helper.RemoveMarkdownChars(txt, "")
					if len(cmd) > 0 {
						message = fmt.Sprintf("*%s*: %s", cmd, conf.MessageUnknownCommand)
					} else {
						message = conf.MessageUnknownCommand
					}
				}
			}
		case StatusWaitingTransmissionUpload:
			switch {
			case strings.HasPrefix(txt, conf.CommandCancel):
				message = conf.MessageCanceled
			default:
				var torrent string
				if update.Message.Document != nil {
					fileResult := b.GetFile(update.Message.Document.FileID)
					torrent = b.GetFileURL(*fileResult.Result)
				} else {
					torrent = txt
				}

				message = transmission.AddTorrent(rpcPort, rpcUsername, rpcPasswd, torrent)
			}

			// reset status
			pool.Sessions[userID] = session{
				UserID:        userID,
				CurrentStatus: StatusWaiting,
			}
		}

		// send message
		if checkMarkdownValidity(message) {
			options["parse_mode"] = bot.ParseModeMarkdown
		}
		if sent := b.SendMessage(update.Message.Chat.ID, message, options); sent.Ok {
			result = true
		} else {
			_stderr.Printf("failed to send message: %s", *sent.Description)

			// log error to db
			db.LogError(*sent.Description)
		}
	} else {
		_stderr.Printf("session does not exist for id: %s", userID)

		// log error to db
		db.LogError(fmt.Sprintf("no session for id: %s", userID))
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

	var message string
	if strings.HasPrefix(txt, conf.CommandCancel) {
		message = ""
	} else if strings.HasPrefix(txt, conf.CommandServiceStart) || strings.HasPrefix(txt, conf.CommandServiceStop) { // service
		message, _ = parseServiceCommand(txt)
	} else if strings.HasPrefix(txt, conf.CommandTransmissionRemove) || strings.HasPrefix(txt, conf.CommandTransmissionDelete) { // transmission
		message, _ = parseTransmissionCommand(txt)
	} else {
		_stderr.Printf("unprocessable callback query: %s", txt)

		db.LogError(fmt.Sprintf("unprocessable callback query: %s", txt))

		return result // == false
	}

	// answer callback query
	options := map[string]interface{}{}
	if len(message) > 0 {
		options["text"] = message
	}
	if apiResult := b.AnswerCallbackQuery(query.ID, options); apiResult.Ok {
		// edit message and remove inline keyboards
		options := map[string]interface{}{
			"chat_id":    query.Message.Chat.ID,
			"message_id": query.Message.MessageID,
		}

		if len(message) <= 0 {
			message = conf.MessageCanceled
		}
		if apiResult := b.EditMessageText(message, options); apiResult.Ok {
			result = true
		} else {
			_stderr.Printf("failed to edit message text: %s", *apiResult.Description)

			db.LogError(fmt.Sprintf("failed to edit message text: %s", *apiResult.Description))
		}
	} else {
		_stderr.Printf("failed to answer callback query: %+v", query)

		db.LogError(fmt.Sprintf("failed to answer callback query: %+v", query))
	}

	return result
}

// broadcast a messge to given chats
func broadcast(client *bot.Bot, chats []helper.Chat, message string) {
	for _, chat := range chats {
		if isAvailableID(chat.UserID) {
			options := defaultOptions()
			if checkMarkdownValidity(message) {
				options["parse_mode"] = bot.ParseModeMarkdown
			}
			if sent := client.SendMessage(
				chat.ChatID,
				message,
				options,
			); !sent.Ok {
				_stderr.Printf("failed to broadcast to chat id %d: %s", chat.ChatID, *sent.Description)

				// log error to db
				db.LogError(*sent.Description)
			}
		} else {
			_stderr.Printf("id not allowed for broadcasting: %s", chat.UserID)

			// log error to db
			db.LogError(fmt.Sprintf("not allowed id for broadcasting: %s", chat.UserID))
		}
	}
}

// for processing incoming request through HTTP
var httpHandler = func(w http.ResponseWriter, r *http.Request) {
	message := strings.TrimSpace(r.FormValue(conf.ParamMessage))

	if len(message) > 0 {
		if isVerbose {
			_stdout.Printf("received message from CLI: %s", message)
		}

		queue <- message
	}
}

// check if given string is valid with markdown characters (true == valid)
func checkMarkdownValidity(txt string) bool {
	if strings.Count(txt, "_")%2 == 0 &&
		strings.Count(txt, "*")%2 == 0 &&
		strings.Count(txt, "`")%2 == 0 &&
		strings.Count(txt, "```")%2 == 0 {
		return true
	}

	return false
}

// default options for messages
func defaultOptions() map[string]interface{} {
	return map[string]interface{}{
		"reply_markup": bot.ReplyKeyboardMarkup{
			Keyboard:       allKeyboards,
			ResizeKeyboard: true,
		},
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
		_stdout.Printf("launching bot: @%s (%s)", *me.Result.Username, me.Result.FirstName)

		// delete webhook (getting updates will not work when wehbook is set up)
		if unhooked := client.DeleteWebhook(false); unhooked.Ok {
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
					cliPort = conf.DefaultCLIPortNumber
				}
				_stdout.Printf("starting local web server for CLI on port: %d", cliPort)

				http.HandleFunc(conf.HTTPBroadcastPath, httpHandler)
				if err := http.ListenAndServe(fmt.Sprintf(":%d", cliPort), nil); err != nil {
					panic(err)
				}
			}()

			// wait for new updates
			client.StartMonitoringUpdates(0, monitorInterval, func(b *bot.Bot, update bot.Update, err error) {
				if err == nil {
					if update.HasMessage() {
						// 'is typing...'
						b.SendChatAction(update.Message.Chat.ID, bot.ChatActionTyping, nil)

						// process message
						processUpdate(b, update)
					} else if update.HasCallbackQuery() {
						// process callback query
						processCallbackQuery(b, update)
					}
				} else {
					_stderr.Printf("error while receiving update (%s)", err)
				}
			})
		} else {
			panic("Failed to delete webhook")
		}
	} else {
		panic("Failed to get info of the bot")
	}
}
