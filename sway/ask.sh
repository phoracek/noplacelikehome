#!/usr/bin/env bash
#
# mod+u: pop up a text prompt, send the question to `claude -p`, show the
# answer as a notification, and copy it to the clipboard.

set -euo pipefail

# sway's exec environment ships a minimal PATH; make sure ~/.local/bin (where
# the claude CLI lives) and ~/bin are reachable.
export PATH="$HOME/.local/bin:$HOME/bin:$PATH"

STACK=string:x-dunst-stack-tag:claude

# Ask the question. Empty stdin turns dmenu-wl into a free-text input box;
# Escape returns non-zero, so just bail out quietly.
query=$(: | dmenu-wl -p "Ask Claude:") || exit 0
[ -z "$query" ] && exit 0

# Let the user know it's working (this notification is replaced by the answer).
dunstify -a claude -h "$STACK" "Asking Claude…" "$query"

# Query from $HOME so no project context (CLAUDE.md etc.) leaks into the answer.
err=$(mktemp)
answer=$(cd "$HOME" && claude -p "$query" --model haiku \
    --system-prompt "You are a desktop quick-answer assistant. Reply in plain text only: no markdown, no backticks. Keep every answer very short, ideally one sentence. If the user asks for a command, output ONLY the raw command on a single line, with no explanation, no alternatives, and no formatting. Never run, execute, or suggest running any commands or tools." \
    2>"$err") || answer=""

if [ -z "$answer" ]; then
    dunstify -a claude -u critical -h "$STACK" "Claude failed" "${query}: $(tail -c 200 "$err")"
    rm -f "$err"
    exit 1
fi
rm -f "$err"

# Stretch goal: copy the answer to the clipboard.
printf '%s' "$answer" | wl-copy || true

# Show the answer, replacing the "Asking…" notification.
dunstify -a claude -t 30000 -h "$STACK" "Claude" "$answer"
