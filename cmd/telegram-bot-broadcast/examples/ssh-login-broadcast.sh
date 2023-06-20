#!/bin/bash
#
# ssh-login-broadcast.sh
#
# created by : meinside@duck.com
# last update: 2023.06.20.
#
# for broadcasting successful ssh logins
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
# 3. open /etc/pam.d/sshd,
#   $ sudo vi /etc/pam.d/sshd
#
# 4. and append following lines (edit path to yours):
#  # for broadcasting to all connected clients on successful logins
#  session optional pam_exec.so seteuid /path/to/this/ssh-login-broadcast.sh

BROADCAST_BIN="/path/to/bin/telegram-bot-broadcast"	# XXX - edit this path

# on session open,
if [ $PAM_TYPE == "open_session" ]; then
	# broadcast
	$BROADCAST_BIN "*sshd >* $PAM_USER has successfully logged into `hostname`, from $PAM_RHOST"
fi
