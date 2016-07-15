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
	MessageDefault                = "Input your command:"
	MessageUnknownCommand         = "Unknown command."
	MessageNoControllableServices = "No controllable services."
	MessageNoLogs                 = "No saved logs."
	MessageServiceToStart         = "Select service to start:"
	MessageServiceToStop          = "Select service to stop:"
	MessageTransmissionUpload     = "Input magnet, url, or file of target torrent:"
	MessageTransmissionRemove     = "Input the number of torrent to remove from the list:"
	MessageTransmissionDelete     = "Input the number of torrent to delete from the list and local storage:"
	MessageTransmissionNoTorrents = "No torrents."
	MessageCanceled               = "Canceled."

	//
	NumRecentLogs = 20
)
