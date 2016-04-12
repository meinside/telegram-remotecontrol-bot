#!/bin/bash
#
# transmission-complete-broadcast.sh
#
# created by : meinside@gmail.com
# last update: 2016.04.12.
#
# for broadcasting transmission download complete message
# through telegram-bot-remotecontrol
# (github.com/meinside/telegram-bot-remotecontrol)
#
# Usage:
#
# 1. setup and run telegram-bot-remotecontrol:
#
# https://github.com/meinside/telegram-bot-remotecontrol
#
# 2. install telegram-bot-broadcast:
#
# $ go get -u github.com/meinside/telegram-bot-remotecontrol/cmd/telegram-bot-broadcast
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
$BROADCAST_BIN "*transmission >* download completed: '$TR_TORRENT_NAME' in $TR_TORRENT_DIR"
