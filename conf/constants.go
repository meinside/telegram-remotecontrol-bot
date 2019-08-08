package conf

const (
	// for CLI
	HttpBroadcastPath    = "/broadcast"
	DefaultCliPortNumber = 59992
	ParamMessage         = "m"
	QueueSize            = 3

	// for Transmission daemon
	DefaultTransmissionRpcPort = 9091

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
	MessageDefault                 = "input your command:"
	MessageUnknownCommand          = "unknown command."
	MessageUnprocessableFileFormat = "unprocessable file format."
	MessageNoControllableServices  = "no controllable services."
	MessageNoLogs                  = "no saved logs."
	MessageServiceToStart          = "select service to start:"
	MessageServiceToStop           = "select service to stop:"
	MessageTransmissionUpload      = "input magnet, url, or file of target torrent:"
	MessageTransmissionRemove      = "input the number of torrent to remove from the list:"
	MessageTransmissionDelete      = "input the number of torrent to delete from the list and local storage:"
	MessageTransmissionNoTorrents  = "no torrents."
	MessageCancel                  = "cancel"
	MessageCanceled                = "canceled."

	//
	NumRecentLogs = 20
)
