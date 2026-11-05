#!/usr/bin/env bash
set -euo pipefail

repo=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
# shellcheck source=installation/backend/lib/aur.sh
source "$repo/installation/backend/lib/aur.sh"

[[ $(browser_role_cmd zen) == zen-browser ]]
[[ $(browser_role_cmd chromium) == chromium ]]
[[ $(browser_role_cmd firefox) == firefox ]]

printf 'installer browser role command: ok\n'
