# Telegram Bot for Controlling Transmission Remotely

You can list/add/remove torrents from your Transmission daemon remotely.

## 0. Prepare

Install go and generate your Telegram bot's API token.

## 1. Install

```bash
$ go get -u github.com/meinside/telegram-bot-go
$ go get -u github.com/meinside/telegram-bot-transmission
$ cd $GOPOATH/src/github.com/meinside/telegram-bot-transmission
$ cp config.json.sample config.json
$ vi config.json
```

and edit values to yours:

```json
{
	"api_token": "0123456789:abcdefghijklmnopqrstuvwyz-x-0a1b2c3d4e",
	"webhook_host": "my.host.somewhere.com",
	"webhook_port": 443,
	"cert_filename": "cert/my_cert.pem",
	"key_filename":"cert/my_cert.key",
	"available_ids": [
		"telegram_id_1",
		"telegram_id_2",
		"telegram_id_3"
	]
}
```

## 2. Build and run

```bash
$ go build -o telegrambot main.go
```

and run it.

## 3. Run as a service

### a. init.d

```bash
$ sudo cp initd/telegrambot-service /etc/init.d/
$ sudo vi /etc/init.d/telegrambot-service
```

and edit **BOT_DIR** value to yours.

If all things are alright, you can start your service now:

```bash
$ sudo service telegrambot-service start
```

or set it up to launch on every boot automatically:

```
$ sudo update-rc.d telegrambot-service defaults
```

### b. systemd

```bash
$ sudo cp systemd/telegrambot.service /lib/systemd/system/
$ sudo vi /lib/systemd/system/telegrambot.service
```

and edit **WorkingDirectory** and **ExecStart** values.

It will launch automatically on boot with:

```bash
$ sudo systemctl enable telegrambot.service
```

and will start with:

```bash
$ sudo systemctl start telegrambot.service
```

## 998. Trouble shooting

If it doesn't work as expected, check if your port is opened publicly. Port should be one of 80, 88, 443, or 8443.

## 999. License

MIT

