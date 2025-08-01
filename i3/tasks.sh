#!/usr/bin/env bash

TASKS_FILE="$HOME/TASKS.md"

function list {
  if [ -f "$TASKS_FILE" ]; then
    cat "$TASKS_FILE"
  fi
}

function edit {
  hx "$TASKS_FILE"
}

function list_undone {
  if [ -f "$TASKS_FILE" ]; then
    grep "^- \[ \] " "$TASKS_FILE" | sed 's/^- \[ \] //'
  fi
}

case "$1" in
  list) list
    ;;
  edit) edit
    ;;
  list-undone) list_undone
    ;;
esac
