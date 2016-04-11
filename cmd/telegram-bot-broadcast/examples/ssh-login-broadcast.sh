#!/bin/bash
#
# ssh-login-broadcast.sh
#
# created by : meinside@gmail.com
# last update: 2016.04.11.
#
# for broadcasting successful ssh logins
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
	$BROADCAST_BIN "sshd > $PAM_USER has successfully logged into `hostname`, from $PAM_RHOST"
fi
