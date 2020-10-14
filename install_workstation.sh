#!/usr/bin/env bash
set -euo pipefail

ansible-playbook play.yml --ask-become-pass --extra-vars '{"workstation": true}'
