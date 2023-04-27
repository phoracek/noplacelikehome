#!/usr/bin/env bash

WORK=/tmp/pomodoro-work
BREAK=/tmp/pomodoro-break

function time_since_touched {
  FILE=$1
  now=$(date +%s)
  modified=$(date -r ${FILE} +%s)
  seconds=$((now - modified))
  echo ${seconds}
}

function work {
  rm -f ${BREAK}
  touch ${WORK}
}

function break {
  rm -f ${WORK}
  touch ${BREAK}
}

function stop {
  rm -f ${WORK} ${BREAK}
}

function status {
  if [ -f "${WORK}" ]; then
    elapsed=$(time_since_touched ${WORK})
    if ((elapsed < 25 * 60)); then
      echo "            work"
    else
      dunstify "Time for a break"
      echo "time for a break"
    fi
  elif [ -f "${BREAK}" ]; then
    elapsed=$(time_since_touched ${BREAK})
    if ((elapsed < 5 * 60)); then
      echo "           break"
    else
      dunstify "Time to work"
      echo "    time to work"
    fi
  else
    echo "                  "
  fi
}

case "$1" in
  work) work
    ;;
  break) break
    ;;
  stop) stop
    ;;
  status) status
    ;;
esac
