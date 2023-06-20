#!/bin/bash
#
# fail2ban-broadcast.sh
#
# created by : meinside@duck.com
# last update: 2023.06.20.
#
# for broadcasting a message on fail2ban's ban action
# through telegram-remotecontrol-bot
# (github.com/meinside/telegram-remotecontrol-bot)
#
# Usage:
#
# 0. install essential packages:
#
# $ sudo apt-get install curl jq
#
# 1. Setup and run telegram-remotecontrol-bot:
#
# https://github.com/meinside/telegram-remotecontrol-bot
#
# 2. Install telegram-bot-broadcast:
#
# $ go get -u github.com/meinside/telegram-remotecontrol-bot/cmd/telegram-bot-broadcast
#
# 3. Duplicate fail2ban's banaction:
#
# $ cd /etc/fail2ban/action.d
# $ sudo cp iptables-multiport.conf iptables-multiport-letmeknow.conf
#
# 4. Append a line at the end of actionban which will execute this bash script:
#
# $ sudo vi iptables-multiport-letmeknow.conf
#
# (example)
# actionban = iptables -I fail2ban-<name> 1 -s <ip> -j DROP
#            /path/to/this/fail2ban-broadcast.sh "<name>" "<ip>"
#
# 5. Change banaction to your newly created one in jail.local:
#
# $ sudo vi /etc/fail2ban/jail.local
#
# ACTIONS
# #banaction = iptables-multiport
# banaction = iptables-multiport-letmeknow
#
# 6. Restart fail2ban service:
#
# $ sudo systemctl restart fail2ban.service

BROADCAST_BIN="/path/to/bin/telegram-bot-broadcast"	# XXX - edit this path

if [ $# -ge 2 ]; then
	PROTOCOL=$1
	IP=$2
	LOCATION=`curl -s http://geoip.nekudo.com/api/$IP | jq '. | .city, .country.name | select(.!=null) | select(.!=false)' | tr '\n' ' '`

	# broadcast
	$BROADCAST_BIN "*fail2ban >* [$PROTOCOL] banned $IP from $LOCATION"
else
	# usage
	echo "$ fail2ban-broadcast.sh PROTOCOL_NAME BANNED_IP (eg. $ fail2ban-broadcast.sh ssh 8.8.8.8)"
fi
