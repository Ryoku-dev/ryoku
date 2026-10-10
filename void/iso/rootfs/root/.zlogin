# shellcheck shell=bash
if [[ "$(tty)" == /dev/tty1 ]]; then
	/usr/local/bin/ryoku-installer-session
fi
