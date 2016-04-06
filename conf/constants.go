package conf

const (
	// for CLI
	HttpBroadcastPath    = "/broadcast"
	DefaultCliPortNumber = 59992
	ParamMessage         = "m"
	QueueSize            = 3

	// for monitoring
	DefaultMonitorIntervalSeconds = 3

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

	// messages
	MessageDefault                = "Input your command:"
	MessageUnknownCommand         = "Unknown command."
	MessageNoControllableServices = "No controllable services."
	MessageControllableServices   = "Available services are:"
	MessageTransmissionUpload     = "Input magnet, url, or file of target torrent:"
	MessageTransmissionRemove     = "Input the number of torrent to remove from the list:"
	MessageTransmissionDelete     = "Input the number of torrent to delete from the list and local storage:"
	MessageCanceled               = "Canceled."
)
