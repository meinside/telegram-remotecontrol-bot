#!/bin/bash
#
# transmission-complete-broadcast.sh
#
# created by : meinside@duck.com
# last update: 2023.06.20.
#
# for broadcasting transmission download complete message
# through telegram-remotecontrol-bot
# (github.com/meinside/telegram-remotecontrol-bot)
#
# Usage:
#
# 1. setup and run telegram-remotecontrol-bot:
#
# https://github.com/meinside/telegram-remotecontrol-bot
#
# 2. install telegram-bot-broadcast:
#
# $ go get -u github.com/meinside/telegram-remotecontrol-bot/cmd/telegram-bot-broadcast
#
# 3. configure Transmission to run this script on download complete:
#
# $ sudo systemctl stop transmission-daemon.service
# $ sudo vi /etc/transmission-daemon/settings.json
#
# # change following values:
# "script-torrent-done-enabled": true,
# "script-torrent-done-filename": "/path/to/this/transmission-complete-broadcast.sh",
#
# $ sudo systemctl start transmission-daemon.service

BROADCAST_BIN="/path/to/bin/telegram-bot-broadcast"	# XXX - edit this path

# broadcast
$BROADCAST_BIN "Transmission > Download completed: '$TR_TORRENT_NAME' in $TR_TORRENT_DIR"
