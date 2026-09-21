#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
bin_dir="$HOME/.local/bin"
unit_dir="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"

install -d "$bin_dir" "$unit_dir"
(cd "$project_root" && go build -trimpath -ldflags='-s -w' -o "$bin_dir/ryoku-palette-bridge" .)
install -m 0755 "$project_root/doctor.sh" "$bin_dir/ryoku-palette-bridge-doctor"
install -m 0755 "$project_root/remove-integrations.sh" "$bin_dir/ryoku-palette-bridge-remove-integrations"
install -m 0644 "$project_root/packaging/systemd/ryoku-palette-bridge.service" \
  "$unit_dir/ryoku-palette-bridge.service"
systemctl --user daemon-reload
for legacy_unit in ryoku-spicetify-palette.service spiceflow.service; do
  if systemctl --user list-unit-files "$legacy_unit" --no-legend 2>/dev/null |
      grep -q "^${legacy_unit}"; then
    systemctl --user disable --now "$legacy_unit"
    printf 'Disabled legacy %s (its unit file was preserved)\n' "$legacy_unit"
  fi
done
printf 'Installed ryoku-palette-bridge.service (enable it when ready).\n'
