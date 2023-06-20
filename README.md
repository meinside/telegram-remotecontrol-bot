# Telegram Bot for Controlling Various Things Remotely

With this bot, you can systemctl start/stop services

and list/add/remove/delete torrents through your Transmission daemon.

## 0. Prepare

Install Go and generate your Telegram bot's API token.

## 1. Install

```bash
$ git clone https://github.com/meinside/telegram-remotecontrol-bot.git
$ cd telegram-remotecontrol-bot
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
	"transmission_rpc_port": 9999,
	"transmission_rpc_username": "some_user",
	"transmission_rpc_passwd": "some_password",
	"cli_port": 59992,
	"is_verbose": false
}
```

When following values are omitted, default values will be applied:

* **monitor_interval**: 3 seconds
* **transmission_rpc_port**: 9091
* **transmission_rpc_username** or **transmission_rpc_passwd**: no username and password (eg. when **rpc-authentication-required** = false)

## 2. Build and run

```bash
$ go build
```

and run it:

```bash
$ ./telegram-remotecontrol-bot
```

## 3. Run as a service

### a. systemd

```bash
$ sudo cp systemd/telegram-remotecontrol-bot.service /lib/systemd/system/
$ sudo vi /lib/systemd/system/telegram-remotecontrol-bot.service
```

and edit **User**, **Group**, **WorkingDirectory** and **ExecStart** values.

It will launch automatically on boot with:

```bash
$ sudo systemctl enable telegram-remotecontrol-bot.service
```

and will start with:

```bash
$ sudo systemctl start telegram-remotecontrol-bot.service
```

## 4. Broadcast to all connected clients

Install command line tool,

```bash
$ go get -u github.com/meinside/telegram-remotecontrol-bot/cmd/telegram-bot-broadcast
```

and type

```bash
$ $GOPATH/bin/telegram-bot-broadcast "SOME_MESSAGE_TO_BROADCAST"
```

then all connected clients who sent at least one message will receive this message.

## 999. License

MIT

