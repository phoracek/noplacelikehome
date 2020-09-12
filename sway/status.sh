#!/usr/bin/env bash

# The Sway configuration file in ~/.config/sway/config calls this script.
# You should see changes to the status bar after saving this script.
# If not, do "killall swaybar" and $mod+Shift+c to reload the configuration.

date_formatted=$(date "+%H:%M")

battery0_capacity=$(cat /sys/class/power_supply/BAT0/capacity)
battery1_capacity=$(cat /sys/class/power_supply/BAT1/capacity)
sum_battery_capacity=$((battery0_capacity / 2 + battery1_capacity / 2 + 1))

storage_used=$(df --output=used -h /dev/dm-2 | sed 1d | sed 's/^[[:space:]]*//')

memory_used=$(free -h | awk 'FNR==2{print $3}')

echo "$date_formatted                                                                                                       $memory_used $storage_used ${sum_battery_capacity}%"
