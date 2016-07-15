# Telegram Bot for Controlling Various Things Remotely

With this bot, you can systemctl start/stop services

and list/add/remove/delete torrents through your Transmission daemon.

## 0. Prepare

Install Go and generate your Telegram bot's API token.

## 1. Install

```bash
$ go get -u github.com/mattn/go-sqlite3
$ go get -u github.com/meinside/telegram-bot-remotecontrol
$ cd $GOPATH/src/github.com/meinside/telegram-bot-remotecontrol
$ cp config.json.sample config.json
$ vi config.json
```

and edit values to yours:

```json
{
	"api_token": "0123456789:abcdefghijklmnopqrstuvwyz-x-0a1b2c3d4e",
	"available_ids": [
		"telegram_id_1",
		"telegram_id_2",
		"telegram_id_3"
	],
	"controllable_services": [
		"vpnserver"
	],
	"monitor_interval": 3,
	"cli_port": 59992,
	"is_verbose": false
}
```

## 2. Build and run

```bash
$ cd $GOPATH/src/github.com/meinside/telegram-bot-remotecontrol
$ go build
```

and run it:

```bash
$ ./telegram-bot-remotecontrol
```

## 3. Run as a service

### a. systemd

```bash
$ sudo cp systemd/telegram-bot-remotecontrol.service /lib/systemd/system/
$ sudo vi /lib/systemd/system/telegram-bot-remotecontrol.service
```

and edit **User**, **Group**, **WorkingDirectory** and **ExecStart** values.

It will launch automatically on boot with:

```bash
$ sudo systemctl enable telegram-bot-remotecontrol.service
```

and will start with:

```bash
$ sudo systemctl start telegram-bot-remotecontrol.service
```

## 4. Broadcast to all connected clients

Install command line tool,

```bash
$ go get -u github.com/meinside/telegram-bot-remotecontrol/cmd/telegram-bot-broadcast
```

and type

```bash
$ $GOPATH/bin/telegram-bot-broadcast "SOME_MESSAGE_TO_BROADCAST"
```

then all connected clients who sent at least one message will receive this message.

## 998. Trouble shooting

TODO

## 999. License

MIT

