#!/usr/bin/env bash

TASKS_FILE="$HOME/TASKS.txt"

function push {
  task=$(echo "" | dmenu -p "Enter task:")
  if [ -n "$task" ]; then
    echo "$task" >> "$TASKS_FILE"
  fi
}

function pop {
  if [ -f "$TASKS_FILE" ]; then
    sed -i '1d' "$TASKS_FILE"
  fi
}

function list {
  if [ -f "$TASKS_FILE" ]; then
    cat "$TASKS_FILE"
  fi
}

function edit {
  hx "$TASKS_FILE"
}

case "$1" in
  push) push
    ;;
  pop) pop
    ;;
  list) list
    ;;
  edit) edit
    ;;
esac