#!/usr/bin/env bash

# Hours and minutes
date_formatted=$(date "+%H:%M")

# Returns the battery capacity (%) and status ("Full", "Discharging", or "Charging")
battery0_capacity=$(cat /sys/class/power_supply/BAT0/capacity)
battery1_capacity=$(cat /sys/class/power_supply/BAT1/capacity)
if [ ! -z "${battery1_capacity}" ]; then
    battery_available=$((battery0_capacity / 2 + battery1_capacity / 2 + 1))
else
    battery_available=$(cat /sys/class/power_supply/BAT0/capacity)
fi

# Storage usage
storage_available=$(df --output=avail -h /dev/dm-0 | sed 1d | sed 's/^[[:space:]]*//')
if [ -z ${storage_available} ]; then
    storage_available=$(df --output=avail -h /dev/dm-1 | sed 1d | sed 's/^[[:space:]]*//')
fi
if [ -z ${storage_available} ]; then
    storage_available=$(df --output=avail -h /dev/dm-2 | sed 1d | sed 's/^[[:space:]]*//')
fi

# Memory usage
memory_available=$(free -h | awk 'FNR==2{print $7}')

# Build the status bar
echo "$date_formatted                                                                                                       ${memory_available} ${storage_available} ${battery_available}%"
