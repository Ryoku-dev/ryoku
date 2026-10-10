#!/bin/sh
set -eu

root=${1:?rootfs path is required}
service_root=$root/etc/runit/runsvdir/default
mkdir -p "$service_root" "$root/var/log" "$root/var/db/xbps/keys"

for service in dbus NetworkManager seatd; do
	[ -d "$root/etc/sv/$service" ] || {
		printf 'post-setup: required live service is missing: %s\n' "$service" >&2
		exit 1
	}
	ln -snf "/etc/sv/$service" "$service_root/$service"
done

# tty1 keeps the stock Void service so login and PAM still establish the root
# session. ttyS0 is an independent root recovery shell for unattended installs.
for getty in agetty-tty1 agetty-ttyS0; do
	[ -x "$root/etc/sv/$getty/run" ] || {
		printf 'post-setup: stock getty service is missing: %s\n' "$getty" >&2
		exit 1
	}
	rm -f "$root/etc/sv/$getty/conf"
done
printf '%s\n' 'GETTY_ARGS="--autologin root --noclear"' \
	> "$root/etc/sv/agetty-tty1/conf"
cat > "$root/etc/sv/agetty-ttyS0/conf" <<'EOF'
GETTY_ARGS="-L -8 --autologin root --noclear"
BAUD_RATE=115200
TERM_NAME=vt100
EOF

chmod 0755 \
	"$root/usr/local/bin/ryoku-installer-session" \
	"$root/usr/local/bin/ryoku-install" \
	"$root/usr/local/bin/ryoku-tui" \
	"$root/usr/local/lib/ryoku/installer-session" \
	"$root/usr/local/lib/ryoku/backend/ryoku-install"
chmod 0644 "$root/etc/sv/agetty-tty1/conf" "$root/etc/sv/agetty-ttyS0/conf"
