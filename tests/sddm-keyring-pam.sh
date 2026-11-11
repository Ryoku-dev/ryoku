#!/usr/bin/env bash
# Verify the SDDM setup wires pam_gnome_keyring once, on any init system.
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
setup=$repo/ryoku/lockscreen/sddm/setup

fail() { printf 'sddm-keyring-pam: %s\n' "$*" >&2; exit 1; }

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

priv() { "$@"; }
# shellcheck disable=SC1090
source <(sed -n '/^wire_keyring_pam()/,/^}/p' "$setup")
declare -F wire_keyring_pam >/dev/null || fail "setup has no wire_keyring_pam"

# wired <name> <input> <want>: wiring <input> must leave exactly <want>.
wired() {
  PAM_FILE=$work/$1
  printf '%s' "$2" >"$PAM_FILE"
  wire_keyring_pam
  [[ $(cat "$PAM_FILE") == "$3" ]] ||
    fail "$1 stack wired wrong:"$'\n'"$(cat "$PAM_FILE")"
}

stock='#%PAM-1.0

auth        include     system-login
-auth       optional    pam_gnome_keyring.so
-auth       optional    pam_kwallet5.so

account     include     system-login

password    include     system-login
-password   optional    pam_gnome_keyring.so    use_authtok

session     optional    pam_keyinit.so          force revoke
session     include     system-login
-session    optional    pam_gnome_keyring.so    auto_start
-session    optional    pam_kwallet5.so         auto_start'
wired stock "$stock" "$stock"

bare='#%PAM-1.0

auth        include     system-login
-auth       optional    pam_kwallet5.so

session     include     system-login
-session    optional    pam_kwallet5.so         auto_start'
wired bare "$bare" '#%PAM-1.0

auth        include     system-login
auth optional pam_gnome_keyring.so
-auth       optional    pam_kwallet5.so

session     include     system-login
session optional pam_gnome_keyring.so auto_start
-session    optional    pam_kwallet5.so         auto_start'

wired again "$(cat "$work/bare")" "$(cat "$work/bare")"

printf 'sddm-keyring-pam: ok\n'
