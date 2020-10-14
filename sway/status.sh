#!/usr/bin/env bash

# Hours and minutes
date_formatted=$(date "+%H:%M")

# Returns the battery capacity (%) and status ("Full", "Discharging", or "Charging")
battery0_capacity=$(cat /sys/class/power_supply/BAT0/capacity)
battery1_capacity=$(cat /sys/class/power_supply/BAT1/capacity)
if [ ! -z "${battery1_capacity}" ]; then
    battery_available=$((battery0_capacity / 2 + battery1_capacity / 2 + 1))
else
    battery_capacity=$(cat /sys/class/power_supply/BAT0/capacity)
fi

# Storage usage
storage_available=$(df --output=avail -h /dev/dm-2 | sed 1d | sed 's/^[[:space:]]*//')

# Memory usage
memory_available=$(free -h | awk 'FNR==2{print $4}')

# Build the status bar
echo "$date_formatted                                                                                                       ${memory_available} ${storage_available} ${battery_available}%"
