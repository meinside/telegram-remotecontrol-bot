package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	bot "github.com/meinside/telegram-bot-go"
	"github.com/meinside/telegram-remotecontrol-bot/cfg"
	"github.com/meinside/telegram-remotecontrol-bot/consts"
	"github.com/meinside/version-go"
)

const (
	githubPageURL = "https://github.com/meinside/telegram-remotecontrol-bot"

	requestTimeoutSeconds          = 60
	ignorableRequestTimeoutSeconds = 5
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

var pool sessionPool

// keyboards
var allKeyboards = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(consts.CommandTransmissionList, consts.CommandTransmissionAdd, consts.CommandTransmissionRemove, consts.CommandTransmissionDelete),
	bot.NewKeyboardButtons(consts.CommandServiceStatus, consts.CommandServiceStart, consts.CommandServiceStop),
	bot.NewKeyboardButtons(consts.CommandStatus, consts.CommandLogs, consts.CommandPrivacy, consts.CommandHelp),
}

var cancelKeyboard = [][]bot.KeyboardButton{
	bot.NewKeyboardButtons(consts.CommandCancel),
}

var (
	_stdout = log.New(os.Stdout, "", log.LstdFlags)
	_stderr = log.New(os.Stderr, "", log.LstdFlags)
)

// check if given Telegram id is available
func isAvailableID(config cfg.Config, id string) bool {
	return slices.Contains(config.AvailableIDs, id)
}

// check if given service is controllable
func isControllableService(controllableServices []string, service string) bool {
	return slices.Contains(controllableServices, service)
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
%s : show privacy policy of this bot
%s : show this help message
`,
		consts.CommandTransmissionList,
		consts.CommandTransmissionAdd,
		consts.CommandTransmissionRemove,
		consts.CommandTransmissionDelete,
		consts.CommandServiceStatus,
		consts.CommandServiceStart,
		consts.CommandServiceStop,
		consts.CommandStatus,
		consts.CommandLogs,
		consts.CommandPrivacy,
		consts.CommandHelp,
	)
}

// for showing privacy policy
func getPrivacyPolicy() string {
	return fmt.Sprintf(`
privacy policy:

%s/raw/master/PRIVACY.md
`, githubPageURL)
}

// get recent logs
func getLogs(db *Database) string {
	var lines []string

	logs := db.GetLogs(consts.NumRecentLogs)

	if len(logs) <= 0 {
		return consts.MessageNoLogs
	}

	for _, log := range logs {
		lines = append(lines, fmt.Sprintf("%s %s: %s", log.CreatedAt.Format("2006-01-02 15:04:05"), log.Type, log.Message))
	}
	return strings.Join(lines, "\n")
}

// for showing current status of this bot
func getStatus(
	config cfg.Config,
	launchedAt time.Time,
) string {
	return fmt.Sprintf("app version: %s\napp uptime: %s\napp memory usage: %s\nsystem disk usage:\n%s",
		version.Minimum(),
		uptimeSince(launchedAt),
		memoryUsage(),
		diskUsage(config.MountPoints),
	)
}

// parse service command and start/stop given service
func parseServiceCommand(
	config cfg.Config,
	db *Database,
	txt string,
) (message string, keyboards [][]bot.InlineKeyboardButton) {
	message = consts.MessageNoControllableServices

	for _, cmd := range []string{consts.CommandServiceStart, consts.CommandServiceStop} {
		if strings.HasPrefix(txt, cmd) {
			service := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

			if isControllableService(config.ControllableServices, service) {
				if strings.HasPrefix(txt, consts.CommandServiceStart) { // start service
					if output, err := systemctlStart(service); err == nil {
						message = fmt.Sprintf("started service: %s", service)
					} else {
						message = fmt.Sprintf("failed to start service: %s (%s)", service, err)

						logError(db, "service failed to start: %s", output)
					}
				} else if strings.HasPrefix(txt, consts.CommandServiceStop) { // stop service
					if output, err := systemctlStop(service); err == nil {
						message = fmt.Sprintf("stopped service: %s", service)
					} else {
						message = fmt.Sprintf("failed to stop service: %s (%s)", service, err)

						logError(db, "service failed to stop: %s", output)
					}
				}
			} else {
				if strings.HasPrefix(txt, consts.CommandServiceStart) { // start service
					message = consts.MessageServiceToStart
				} else if strings.HasPrefix(txt, consts.CommandServiceStop) { // stop service
					message = consts.MessageServiceToStop
				}

				keys := map[string]string{}
				for _, v := range config.ControllableServices {
					keys[v] = fmt.Sprintf("%s %s", cmd, v)
				}
				keyboards = bot.NewInlineKeyboardButtonsAsRowsWithCallbackData(keys)

				// add cancel button
				keyboards = append(keyboards, []bot.InlineKeyboardButton{
					bot.NewInlineKeyboardButton(consts.MessageCancel).
						SetCallbackData(consts.CommandCancel),
				})
			}
		}
		continue
	}

	return message, keyboards
}

// parse transmission command
func parseTransmissionCommand(
	config cfg.Config,
	txt string,
) (message string, keyboards [][]bot.InlineKeyboardButton) {
	if torrents, _ := GetTorrents(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd); len(torrents) > 0 {
		for _, cmd := range []string{consts.CommandTransmissionRemove, consts.CommandTransmissionDelete} {
			if strings.HasPrefix(txt, cmd) {
				param := strings.TrimSpace(strings.Replace(txt, cmd, "", 1))

				if _, err := strconv.Atoi(param); err == nil { // if torrent id number is given,
					if strings.HasPrefix(txt, consts.CommandTransmissionRemove) { // remove torrent
						message = RemoveTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, param)
					} else if strings.HasPrefix(txt, consts.CommandTransmissionDelete) { // delete service
						message = DeleteTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, param)
					}
				} else {
					if strings.HasPrefix(txt, consts.CommandTransmissionRemove) { // remove torrent
						message = consts.MessageTransmissionRemove
					} else if strings.HasPrefix(txt, consts.CommandTransmissionDelete) { // delete torrent
						message = consts.MessageTransmissionDelete
					}

					// inline keyboards
					keys := map[string]string{}
					for _, t := range torrents {
						keys[fmt.Sprintf("%d. %s", t.ID, t.Name)] = fmt.Sprintf("%s %d", cmd, t.ID)
					}
					keyboards = bot.NewInlineKeyboardButtonsAsRowsWithCallbackData(keys)

					// add cancel button
					keyboards = append(keyboards, []bot.InlineKeyboardButton{
						bot.NewInlineKeyboardButton(consts.MessageCancel).
							SetCallbackData(consts.CommandCancel),
					})
				}
			}
			continue
		}
	} else {
		message = consts.MessageTransmissionNoTorrents
	}

	return message, keyboards
}

// process incoming update from Telegram
func processUpdate(
	ctx context.Context,
	b *bot.Bot,
	config cfg.Config,
	db *Database,
	launchedAt time.Time,
	update bot.Update,
) bool {
	// check username
	var userID string
	if from := update.GetFrom(); from == nil {
		logError(db, "update has no 'from' value")

		return false
	} else {
		if from.Username == nil {
			logError(db, "has no user name: %s", from.FirstName)

			return false
		}
		userID = *from.Username
		if !isAvailableID(config, userID) {
			logError(db, "not an allowed user id: %s", userID)

			return false
		}
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
		options := bot.OptionsSendMessage{}.
			SetReplyMarkup(defaultReplyMarkup(true))

		switch s.CurrentStatus {
		case StatusWaiting:
			if update.Message.Document != nil { // if a file is received,
				// get file info
				ctxFileInfo, cancelFileInfo := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
				defer cancelFileInfo()
				fileResult, _ := b.GetFile(ctxFileInfo, update.Message.Document.FileID)

				fileURL := b.GetFileURL(*fileResult.Result)

				// XXX - only support: .torrent
				if strings.HasSuffix(fileURL, ".torrent") {
					addReaction(ctx, b, update, "👌")
					message = AddTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, fileURL)
				} else {
					message = consts.MessageUnprocessableFileFormat
				}
			} else {
				switch {
				// magnet url
				case strings.HasPrefix(txt, "magnet:"):
					addReaction(ctx, b, update, "👌")
					message = AddTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, txt)
				// /start
				case strings.HasPrefix(txt, consts.CommandStart):
					message = consts.MessageDefault
				// systemctl
				case strings.HasPrefix(txt, consts.CommandServiceStatus):
					statuses, _ := systemctlStatus(config.ControllableServices)
					for service, status := range statuses {
						message += fmt.Sprintf("┖ %s: *%s*\n", service, status)
					}
				case strings.HasPrefix(txt, consts.CommandServiceStart) || strings.HasPrefix(txt, consts.CommandServiceStop):
					if len(config.ControllableServices) > 0 {
						var keyboards [][]bot.InlineKeyboardButton
						message, keyboards = parseServiceCommand(config, db, txt)
						if keyboards != nil {
							options.SetReplyMarkup(bot.NewInlineKeyboardMarkup(keyboards))
						}
					} else {
						message = consts.MessageNoControllableServices
					}
				// transmission
				case strings.HasPrefix(txt, consts.CommandTransmissionList):
					message = GetList(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd)
				case strings.HasPrefix(txt, consts.CommandTransmissionAdd):
					arg := strings.TrimSpace(strings.Replace(txt, consts.CommandTransmissionAdd, "", 1))
					if strings.HasPrefix(arg, "magnet:") {
						message = AddTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, arg)
					} else {
						message = consts.MessageTransmissionUpload
						pool.Sessions[userID] = session{
							UserID:        userID,
							CurrentStatus: StatusWaitingTransmissionUpload,
						}
						options.SetReplyMarkup(cancelReplyMarkup(true))
					}
				case strings.HasPrefix(txt, consts.CommandTransmissionRemove) || strings.HasPrefix(txt, consts.CommandTransmissionDelete):
					var keyboards [][]bot.InlineKeyboardButton
					message, keyboards = parseTransmissionCommand(config, txt)
					if keyboards != nil {
						options.SetReplyMarkup(bot.NewInlineKeyboardMarkup(keyboards))
					}
				case strings.HasPrefix(txt, consts.CommandStatus):
					message = getStatus(config, launchedAt)
				case strings.HasPrefix(txt, consts.CommandLogs):
					message = getLogs(db)
				case strings.HasPrefix(txt, consts.CommandHelp):
					message = getHelp()
					options.SetReplyMarkup(helpInlineKeyboardMarkup())
				case strings.HasPrefix(txt, consts.CommandPrivacy):
					message = getPrivacyPolicy()
				// fallback
				default:
					cmd := removeMarkdownChars(txt, "")
					if len(cmd) > 0 {
						message = fmt.Sprintf("*%s*: %s", cmd, consts.MessageUnknownCommand)
					} else {
						message = consts.MessageUnknownCommand
					}
				}
			}
		case StatusWaitingTransmissionUpload:
			switch {
			case strings.HasPrefix(txt, consts.CommandCancel):
				message = consts.MessageCanceled
			default:
				var torrent string
				if update.Message.Document != nil {
					// get file info
					ctxFileInfo, cancelFileInfo := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
					defer cancelFileInfo()
					fileResult, _ := b.GetFile(ctxFileInfo, update.Message.Document.FileID)

					torrent = b.GetFileURL(*fileResult.Result)
				} else {
					torrent = txt
				}

				addReaction(ctx, b, update, "👌")
				message = AddTorrent(config.TransmissionRPCPort, config.TransmissionRPCUsername, config.TransmissionRPCPasswd, torrent)
			}

			// reset status
			pool.Sessions[userID] = session{
				UserID:        userID,
				CurrentStatus: StatusWaiting,
			}
		}

		// send message
		ctxSend, cancelSend := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
		defer cancelSend()
		if checkMarkdownValidity(message) {
			options.SetParseMode(bot.ParseModeMarkdown)
		}
		if sent, err := b.SendMessage(
			ctxSend,
			update.Message.Chat.ID,
			message,
			options,
		); sent.OK {
			result = true
		} else {
			var errMessageEmpty bot.ErrMessageEmpty
			var errMessageTooLong bot.ErrMessageTooLong
			var errNoChatID bot.ErrChatNotFound
			var errTooManyRequests bot.ErrTooManyRequests
			if errors.As(err, &errMessageEmpty) {
				logError(db, "message is empty")
			} else if errors.As(err, &errMessageTooLong) {
				logError(db, "message is too long: %d bytes", len(message))
			} else if errors.As(err, &errNoChatID) {
				logError(db, "no such chat id: %d", update.Message.Chat.ID)
			} else if errors.As(err, &errTooManyRequests) {
				logError(db, "too many requests")
			} else {
				logError(db, "failed to send message: %s", *sent.Description)
			}
		}
	} else {
		logError(db, "no session for id: %s", userID)
	}
	pool.Unlock()

	return result
}

// add reaction to a message
func addReaction(
	ctx context.Context,
	b *bot.Bot,
	update bot.Update,
	reaction string,
) {
	if !update.HasMessage() {
		return
	}

	chatID := update.Message.Chat.ID
	messageID := update.Message.MessageID

	// add reaction
	ctxReaction, cancelReaction := context.WithTimeout(ctx, ignorableRequestTimeoutSeconds*time.Second)
	defer cancelReaction()
	_, _ = b.SetMessageReaction(ctxReaction, chatID, messageID, bot.NewMessageReactionWithEmoji(reaction))
}

// process incoming callback query
func processCallbackQuery(
	ctx context.Context,
	b *bot.Bot,
	config cfg.Config,
	db *Database,
	update bot.Update,
) (result bool) {
	query := *update.CallbackQuery
	txt := *query.Data

	// process result
	result = false

	var message string
	if strings.HasPrefix(txt, consts.CommandCancel) {
		message = ""
	} else if strings.HasPrefix(txt, consts.CommandServiceStart) || strings.HasPrefix(txt, consts.CommandServiceStop) { // service
		message, _ = parseServiceCommand(config, db, txt)
	} else if strings.HasPrefix(txt, consts.CommandTransmissionRemove) || strings.HasPrefix(txt, consts.CommandTransmissionDelete) { // transmission
		message, _ = parseTransmissionCommand(config, txt)
	} else {
		logError(db, "unprocessable callback query: %s", txt)

		return result
	}

	// answer callback query
	options := bot.OptionsAnswerCallbackQuery{}
	if len(message) > 0 {
		options.SetText(message)
	}
	ctxAnswer, cancelAnswer := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
	defer cancelAnswer()
	if apiResult, _ := b.AnswerCallbackQuery(ctxAnswer, query.ID, options); apiResult.OK {
		if len(message) <= 0 {
			message = consts.MessageCanceled
		}

		// edit message and remove inline keyboards
		ctxEdit, cancelEdit := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
		defer cancelEdit()
		if apiResult, _ := b.EditMessageText(
			ctxEdit,
			message,
			bot.OptionsEditMessageText{}.
				SetIDs(query.Message.Chat.ID, query.Message.MessageID),
		); apiResult.OK {
			result = true
		} else {
			logError(db, "failed to edit message text: %s", *apiResult.Description)
		}
	} else {
		logError(db, "failed to answer callback query: %+v", query)
	}

	return result
}

// broadcast a messge to given chats
func broadcast(
	ctx context.Context,
	client *bot.Bot,
	config cfg.Config,
	db *Database,
	message string,
) {
	for _, chat := range db.GetChats() {
		if isAvailableID(config, chat.UserID) {
			options := bot.OptionsSendMessage{}.
				SetReplyMarkup(defaultReplyMarkup(true))
			if checkMarkdownValidity(message) {
				options.SetParseMode(bot.ParseModeMarkdown)
			}
			ctxSend, cancelSend := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
			defer cancelSend()
			if sent, err := client.SendMessage(
				ctxSend,
				chat.ChatID,
				message,
				options,
			); !sent.OK {
				var errMessageEmpty bot.ErrMessageEmpty
				var errMessageTooLong bot.ErrMessageTooLong
				var errNoChatID bot.ErrChatNotFound
				var errTooManyRequests bot.ErrTooManyRequests
				if errors.As(err, &errMessageEmpty) {
					logError(db, "broadcast message is empty")
				} else if errors.As(err, &errMessageTooLong) {
					logError(db, "broadcast message is too long: %d bytes", len(message))
				} else if errors.As(err, &errNoChatID) {
					logError(db, "no such chat id for broadcast: %d", chat.ChatID)
				} else if errors.As(err, &errTooManyRequests) {
					logError(db, "too many requests for broadcast")
				} else {
					logError(db, "failed to broadcast to chat id %d: %s", chat.ChatID, *sent.Description)
				}
			}
		} else {
			logError(db, "not an allowed user id for boradcasting: %s", chat.UserID)
		}
	}
}

// for processing incoming request through HTTP
func httpHandlerForCLI(config cfg.Config, queue chan string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		message := strings.TrimSpace(r.FormValue(consts.ParamMessage))

		if len(message) > 0 {
			if config.IsVerbose {
				_stdout.Printf("received message from CLI: %s", message)
			}

			queue <- message
		}
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

// default reply markup for messages
func defaultReplyMarkup(resize bool) bot.ReplyKeyboardMarkup {
	return bot.NewReplyKeyboardMarkup(allKeyboards).
		SetResizeKeyboard(resize)
}

// reply markup for cancel
func cancelReplyMarkup(resize bool) bot.ReplyKeyboardMarkup {
	return bot.NewReplyKeyboardMarkup(cancelKeyboard).
		SetResizeKeyboard(resize)
}

// inline keyboard markup for help
func helpInlineKeyboardMarkup() bot.InlineKeyboardMarkup {
	return bot.NewInlineKeyboardMarkup( // inline keyboard for link to github page
		[][]bot.InlineKeyboardButton{
			bot.NewInlineKeyboardButtonsWithURL(map[string]string{
				"GitHub": githubPageURL,
			}),
		},
	)
}

// run bot
func runBot(
	ctx context.Context,
	config cfg.Config,
	launchedAt time.Time,
) {
	// initialize variables
	sessions := make(map[string]session)
	for _, v := range config.AvailableIDs {
		sessions[v] = session{
			UserID:        v,
			CurrentStatus: StatusWaiting,
		}
	}
	pool = sessionPool{
		Sessions: sessions,
	}
	queue := make(chan string, consts.QueueSize)

	// open database
	db, err := OpenDB()
	if err != nil {
		_stderr.Fatalf("failed to open database: %s", err)
	}

	db.Log("starting server...")

	// catch SIGINT and SIGTERM and terminate gracefully
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		db.Log("stopping server...")
		os.Exit(1)
	}()

	client := bot.NewClient(config.APIToken)
	client.Verbose = config.IsVerbose

	// get info about this bot
	ctxBotInfo, cancelBotInfo := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
	defer cancelBotInfo()
	if me, _ := client.GetMe(ctxBotInfo); me.OK {
		_stdout.Printf("launching bot: @%s (%s)", *me.Result.Username, me.Result.FirstName)

		// delete webhook (getting updates will not work when wehbook is set up)
		ctxDeleteWebhook, cancelDeleteWebhook := context.WithTimeout(ctx, requestTimeoutSeconds*time.Second)
		defer cancelDeleteWebhook()
		if unhooked, _ := client.DeleteWebhook(ctxDeleteWebhook, false); unhooked.OK {
			// wait for CLI message channel
			go func() {
				// broadcast messages from CLI
				for message := range queue {
					broadcast(ctx, client, config, db, message)
				}
			}()

			// start web server for CLI
			go func(config cfg.Config) {
				if config.CLIPort <= 0 {
					config.CLIPort = consts.DefaultCLIPortNumber
				}
				_stdout.Printf("starting local web server for CLI on port: %d", config.CLIPort)

				http.HandleFunc(consts.HTTPBroadcastPath, httpHandlerForCLI(config, queue))
				if err := http.ListenAndServe(fmt.Sprintf(":%d", config.CLIPort), nil); err != nil {
					panic(err)
				}
			}(config)

			// set update handlers
			client.SetMessageHandler(func(b *bot.Bot, update bot.Update, message bot.Message, edited bool) {
				// 'is typing...'
				ctxAction, cancelAction := context.WithTimeout(ctx, ignorableRequestTimeoutSeconds*time.Second)
				defer cancelAction()
				_, _ = b.SendChatAction(ctxAction, message.Chat.ID, bot.ChatActionTyping, nil)

				// process message
				processUpdate(ctx, b, config, db, launchedAt, update)
			})
			client.SetCallbackQueryHandler(func(b *bot.Bot, update bot.Update, callbackQuery bot.CallbackQuery) {
				// 'is typing...'
				ctxAction, cancelAction := context.WithTimeout(ctx, ignorableRequestTimeoutSeconds*time.Second)
				defer cancelAction()
				_, _ = b.SendChatAction(ctxAction, callbackQuery.Message.Chat.ID, bot.ChatActionTyping, nil)

				// process callback query
				processCallbackQuery(ctx, b, config, db, update)
			})

			// wait for new updates
			client.StartPollingUpdates(0, config.MonitorInterval, func(b *bot.Bot, update bot.Update, err error) {
				if err != nil {
					logError(db, "error while receiving update: %s", err)
				}
			})
		} else {
			panic("Failed to delete webhook")
		}
	} else {
		panic("Failed to get info of the bot")
	}
}

// log error to stderr and DB
func logError(db *Database, format string, a ...any) {
	_stderr.Printf(format, a...)

	db.LogError(fmt.Sprintf(format, a...))
}
