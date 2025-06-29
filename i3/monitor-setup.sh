#!/usr/bin/env bash

# Monitor setup script for DP-1-3 display with integrated udev wrapper
# Automatically configures display based on DP-1-3 connection and laptop lid status

# If running as root (from udev), re-execute as user with proper environment
if [ "$(id -u)" -eq 0 ]; then
    TARGET_USER="phoracek"
    
    # Find the user's X display
    USER_DISPLAY=""
    for display_file in /tmp/.X11-unix/X*; do
        if [ -S "$display_file" ]; then
            display_num=${display_file##*/X}
            if ps aux | grep -q "Xorg.*:$display_num.*$TARGET_USER\|gdm.*:$display_num"; then
                USER_DISPLAY=":$display_num"
                break
            fi
        fi
    done
    
    [ -z "$USER_DISPLAY" ] && exit 1
    
    # Re-execute as user with proper environment
    su - "$TARGET_USER" -c "
        export DISPLAY='$USER_DISPLAY'
        export XDG_RUNTIME_DIR='/run/user/$(id -u $TARGET_USER)'
        '$0'
    " >/dev/null 2>&1
    exit 0
fi

# Normal execution as user

# Function to log messages
log_message() {
    LOG_FILE="$HOME/.config/i3/monitor.log"
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" >> "$LOG_FILE"
}

# Set up DISPLAY if not already set
[ -z "$DISPLAY" ] && export DISPLAY=:0

# Check if laptop lid is closed
is_lid_closed() {
    grep -q "closed" /proc/acpi/button/lid/*/state 2>/dev/null
}

# Check if DP-1-3 display is connected
is_dp1_3_connected() {
    xrandr | grep -q "DP-1-3 connected"
}

# Configure displays based on connection and lid status
if is_dp1_3_connected && is_lid_closed; then
    # External display only (lid closed)
    log_message "DP-1-3 connected, lid closed - external display only"
    xrandr --output eDP-1 --off --output DP-1-3 --auto
    notify-send "Monitor" "External display activated" 2>/dev/null || true
elif is_dp1_3_connected; then
    # Both displays (lid open or lid state unknown)
    log_message "DP-1-3 connected, lid open - dual display"
    xrandr --output eDP-1 --auto --output DP-1-3 --auto
    notify-send "Monitor" "Dual display activated" 2>/dev/null || true
else
    # Laptop display only (DP-1-3 not connected)
    log_message "DP-1-3 not connected - laptop display only"
    xrandr --output DP-1-3 --off --output eDP-1 --auto
    notify-send "Monitor" "Laptop display only" 2>/dev/null || true
fi
