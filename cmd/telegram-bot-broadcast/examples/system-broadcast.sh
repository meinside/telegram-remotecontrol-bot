#!/bin/bash
#
# system-broadcast.sh
#
# created by : meinside@gmail.com
# last update: 2016.04.11.
#
# for broadcasting system status
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
# 3. register this script on crontab

BROADCAST_BIN="/path/to/bin/telegram-bot-broadcast"	# XXX - edit this

HOSTNAME=`hostname`
IP_ADDR=`hostname -I`
UNAME=`uname -a`
UPTIME=`uptime`
DF=`df -h`
TEMP=`vcgencmd measure_temp`
MEMORY=`free -o -h`

# message
MSG="system status: $HOSTNAME ($IP_ADDR)

$UNAME

$UPTIME

$DF

$TEMP

$MEMORY"

# broadcast
$BROADCAST_BIN "$MSG"
