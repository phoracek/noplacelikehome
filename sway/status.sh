#!/usr/bin/env bash

# The Sway configuration file in ~/.config/sway/config calls this script.
# You should see changes to the status bar after saving this script.
# If not, do "killall swaybar" and $mod+Shift+c to reload the configuration.

# Produces "21 days", for example
uptime_formatted=$(uptime | cut -d ',' -f1 | cut -d ' ' -f4,5 | sed 's/^[[:space:]]*//')

# The ISO-formatted date like 2018-10-06 and the time (e.g., 14:01)
date_formatted=$(date "+%F %H:%M")

# Returns the battery capacity (%) and status ("Full", "Discharging", or "Charging").
battery0_capacity=$(cat /sys/class/power_supply/BAT0/capacity)
battery0_status=$(cat /sys/class/power_supply/BAT0/status)
battery1_capacity=$(cat /sys/class/power_supply/BAT1/capacity)
battery1_status=$(cat /sys/class/power_supply/BAT1/status)

# Storage usage
storage_total=$(df --output=size -h /dev/dm-2 | sed 1d | sed 's/^[[:space:]]*//')
storage_used=$(df --output=used -h /dev/dm-2 | sed 1d | sed 's/^[[:space:]]*//')

# Memory usage
memory_total=$(free -h | awk 'FNR==2{print $2}')
memory_used=$(free -h | awk 'FNR==2{print $3}')

# Keyboard layout
layout=$(setxkbmap -query | grep layout | awk '{print $2}')

# Sound stats
master_status=$(amixer sget Master | grep '\[on\]' &>/dev/null && echo '+' || echo '-')
master_volume=$(amixer sget Master | grep 'Front Left: ' | sed 's/.*\[\(.*\)\%\].*/\1/')
capture_status=$(amixer sget Capture | grep '\[on\]' &>/dev/null && echo '+' || echo '-')

# Emojis and characters for the status bar
# echo "up:[$uptime_formatted] m:[$memory_used/$memory_total] s:[$storage_used/$storage_total] b0:[$battery0_capacity;$battery0_status] b1:[$battery1_capacity;$battery1_status] v:[p$master_volume$master_status;c$capture_status] k:[$layout] t:[$date_formatted]"
sum_cap=$((battery0_capacity / 2 + battery1_capacity / 2 + 1))
date_formatted=$(date "+%H:%M")
echo "$date_formatted                                                                                                       $memory_used $storage_used ${sum_cap}%"
