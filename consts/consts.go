package consts

// constants
const (
	// for CLI
	HTTPBroadcastPath    = "/broadcast"
	DefaultCLIPortNumber = 59992
	ParamMessage         = "m"
	QueueSize            = 3

	// for Transmission daemon
	DefaultTransmissionRPCPort = 9091

	// for monitoring
	DefaultMonitorIntervalSeconds = 3

	// commands
	CommandStart  = "/start"
	CommandStatus = "/status"
	CommandLogs   = "/logs"
	CommandHelp   = "/help"
	CommandCancel = "/cancel"

	// commands for systemctl
	CommandServiceStatus = "/servicestatus"
	CommandServiceStart  = "/servicestart"
	CommandServiceStop   = "/servicestop"

	// commands for transmission
	CommandTransmissionList   = "/trlist"
	CommandTransmissionAdd    = "/tradd"
	CommandTransmissionRemove = "/trremove"
	CommandTransmissionDelete = "/trdelete"

	// messages
	MessageDefault                 = "Input your command:"
	MessageUnknownCommand          = "Unknown command."
	MessageUnprocessableFileFormat = "Unprocessable file format."
	MessageNoControllableServices  = "No controllable services."
	MessageNoLogs                  = "No saved logs."
	MessageServiceToStart          = "Select service to start:"
	MessageServiceToStop           = "Select service to stop:"
	MessageTransmissionUpload      = "Send magnet, url, or file of target torrent:"
	MessageTransmissionRemove      = "Send the id of torrent to remove from the list:"
	MessageTransmissionDelete      = "Send the id of torrent to delete from the list and local storage:"
	MessageTransmissionNoTorrents  = "No torrents."
	MessageCancel                  = "Cancel"
	MessageCanceled                = "Canceled."

	// number of recent logs
	NumRecentLogs = 20
)
