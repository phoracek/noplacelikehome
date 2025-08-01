#!/usr/bin/env bash

TASKS_FILE="$HOME/TASKS.txt"

function list {
  if [ -f "$TASKS_FILE" ]; then
    cat "$TASKS_FILE"
  fi
}

function edit {
  hx "$TASKS_FILE"
}

case "$1" in
  list) list
    ;;
  edit) edit
    ;;
esac
