// telegram bot for controlling transmission remotely
package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/profile"

	bot "github.com/meinside/telegram-bot-go"

	"github.com/meinside/telegram-bot-remotecontrol/conf"
	"github.com/meinside/telegram-bot-remotecontrol/services"
	"github.com/meinside/telegram-bot-remotecontrol/services/transmission"
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
	CliPort              int      `json:"cli_port"`
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

	// commands for systemctl
	CommandServiceStart = "/servicestart"
	CommandServiceStop  = "/servicestop"

	// commands for transmission
	CommandTransmissionList   = "/trlist"
	CommandTransmissionAdd    = "/tradd"
	CommandTransmissionRemove = "/trremove"
	CommandTransmissionDelete = "/trdelete"
)

const (
	// messages
	DefaultMessage                = "Input your command:"
	MessageUnknownCommand         = "Unknown command."
	MessageNoControllableServices = "No controllable services."
	MessageControllableServices   = "Available services are:"
	MessageTransmissionUpload     = "Input magnet, url, or file of target torrent:"
	MessageTransmissionRemove     = "Input the number of torrent to remove from the list:"
	MessageTransmissionDelete     = "Input the number of torrent to delete from the list and local storage:"
	MessageCanceled               = "Canceled."
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

// keyboards
var allKeyboards = [][]string{
	[]string{CommandServiceStart, CommandServiceStop},
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
	if config, err := conf.GetConfig(); err == nil {
		apiToken = config.ApiToken
		availableIds = config.AvailableIds
		controllableServices = config.ControllableServices
		monitorInterval = config.MonitorInterval
		if monitorInterval <= 0 {
			monitorInterval = DefaultMonitorIntervalSeconds
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

*For Systemctl*

/servicestart _SERVICE_ : start a service
/servicestop _SERVICE_ : stop a service

*For Transmission*

/trlist : show torrent list
/tradd : add torrent with url or magnet
/trremove : remove torrent from list
/trdelete : remove torrent and delete data

*Others*

/status : show this bot's status
/help : show this help message
`
}

// for showing current status of this bot
func getStatus() string {
	return fmt.Sprintf("Uptime: %s\nMemory Usage: %s", getUptime(launched), getMemoryUsage())
}

// parse service command
func parseServiceCommand(txt string) (message string, keyboards [][]string) {
	message = MessageNoControllableServices
	keyboards = nil

	for _, cmd := range []string{CommandServiceStart, CommandServiceStop} {
		if strings.HasPrefix(txt, cmd) {
			service := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

			if isControllableService(service) {
				if strings.HasPrefix(txt, CommandServiceStart) { // start service
					if output, ok := services.Start(service); ok {
						message = fmt.Sprintf("Started service: *%s*", service)
					} else {
						message = output
					}
				} else if strings.HasPrefix(txt, CommandServiceStop) { // stop service
					if output, ok := services.Stop(service); ok {
						message = fmt.Sprintf("Stopped service: *%s*", service)
					} else {
						message = output
					}
				}
			} else {
				message = MessageControllableServices

				keys := []string{}
				for _, v := range controllableServices {
					keys = append(keys, fmt.Sprintf("%s %s", cmd, v))
				}

				keyboards = [][]string{
					keys,
					[]string{CommandCancel},
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
		return false
	}

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
			case strings.HasPrefix(txt, CommandStart):
				message = DefaultMessage
			// systemctl
			case strings.HasPrefix(txt, CommandServiceStart) || strings.HasPrefix(txt, CommandServiceStop):
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
					message = MessageNoControllableServices
				}
			// transmission
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
			case strings.HasPrefix(txt, CommandStatus):
				message = getStatus()
			case strings.HasPrefix(txt, CommandHelp):
				message = getHelp()
			// fallback
			default:
				message = fmt.Sprintf("*%s*: %s", txt, MessageUnknownCommand)
			}
		case StatusWaitingServiceName:
			switch {
			// systemctl
			case strings.HasPrefix(txt, CommandServiceStart) || strings.HasPrefix(txt, CommandServiceStop):
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
					message = MessageNoControllableServices
				}
			// cancel
			default:
				message = MessageCanceled
			}

			// reset status
			pool.Sessions[userId] = Session{
				UserId:        userId,
				CurrentStatus: StatusWaiting,
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
	} else {
		log.Printf("*** Session does not exist for id: %s\n", userId)
	}

	pool.Unlock()

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
					case s := <-queue:
						// TODO - broadcast to all connected chat ids
						log.Printf("received message from queue: %s\n", s)
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
					panic(err.Error())
				}
			}()

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
