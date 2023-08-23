# Telegram Bot for Controlling Various Things Remotely

With this bot, you can systemctl start/stop services

and list/add/remove/delete torrents through your Transmission daemon.

## 0. Prepare

Install Go and [generate your Telegram bot's API token](https://telegram.me/BotFather).

## 1. Install

```bash
$ git clone https://github.com/meinside/telegram-remotecontrol-bot.git
$ cd telegram-remotecontrol-bot
$ go build
```

or

```bash
$ go install github.com/meinside/telegram-remotecontrol-bot@latest
```

## 2. Configure

Put your `config.json` file in `$XDG_CONFIG_HOME/telegram-remotecontrol-bot/` directory:

```bash
$ mkdir -p ~/.config/telegram-remotecontrol-bot/
$ cp config.json.sample ~/.config/telegram-remotecontrol-bot/config.json
$ vi config.json
```

and edit values to yours:

```json
{
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
  "is_verbose": false,

  "api_token": "0123456789:abcdefghijklmnopqrstuvwyz-x-0a1b2c3d4e"
}
```

When following values are omitted, default values will be applied:

* **monitor_interval**: 3 seconds
* **transmission_rpc_port**: 9091
* **transmission_rpc_username** or **transmission_rpc_passwd**: no username and password (eg. when **rpc-authentication-required** = false)

### Using Infisical

You can also use [Infisical](https://infisical.com/) for retrieving your bot api token:

```json
{
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
  "is_verbose": false,

  "infisical": {
    "workspace_id": "012345abcdefg",
    "token": "st.xyzwabcd.0987654321.abcdefghijklmnop",
    "environment": "dev",
    "secret_type": "shared",

    "api_token_key_path": "/path/to/your/KEY_TO_API_TOKEN"
  }
}
```

If your Infisical workspace's E2EE setting is enabled, you also need to provide your API key:

```json
{
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
  "is_verbose": false,

  "infisical": {
    "e2ee": true,
    "api_key": "ak.1234567890.abcdefghijk",

    "workspace_id": "012345abcdefg",
    "token": "st.xyzwabcd.0987654321.abcdefghijklmnop",
    "environment": "dev",
    "secret_type": "shared",

    "api_token_key_path": "/path/to/your/KEY_TO_API_TOKEN"
  }
}
```

## 3. Run

Run the built(or installed) binary with:

```bash
$ ./telegram-remotecontrol-bot
# or
$ $(go env GOPATH)/bin/telegram-remotecontrol-bot

```

## 4. Run as a service

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
$ go install github.com/meinside/telegram-remotecontrol-bot/cmd/telegram-bot-broadcast@latest
```

and run:

```bash
$ $(go env GOPATH)/bin/telegram-bot-broadcast "SOME_MESSAGE_TO_BROADCAST"
```

then all connected clients who sent at least one message will receive this message.

## 999. License

MIT

