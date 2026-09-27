#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root=$(mktemp -d /tmp/ryoku-palette-core-install.XXXXXX)
trap 'rm -rf "$test_root"' EXIT
fake_bin="$test_root/bin"
systemctl_log="$test_root/systemctl.log"
mkdir -p "$fake_bin" "$test_root/home/.config/systemd/user"

cat > "$fake_bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
output=
[[ -f go.mod ]] || { printf 'build started outside the Go module\n' >&2; exit 1; }
while (($#)); do
  if [[ $1 == -o ]]; then
    output=$2
    break
  fi
  shift
done
[[ -n $output ]]
printf '#!/usr/bin/env bash\nexit 0\n' > "$output"
chmod +x "$output"
EOF

cat > "$fake_bin/systemctl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_SYSTEMCTL_LOG"
if [[ $* == *list-unit-files* ]]; then
  unit=${3:-}
  case $unit in
    ryoku-spicetify-palette.service|spiceflow.service)
      printf '%s enabled\n' "$unit"
      ;;
  esac
fi
EOF
printf '#!/usr/bin/env bash\nexit 0\n' > "$fake_bin/ryoku"
printf '#!/usr/bin/env bash\nexit 0\n' > "$fake_bin/ryogami"
chmod +x "$fake_bin/go" "$fake_bin/systemctl" "$fake_bin/ryoku" "$fake_bin/ryogami"

(cd "$test_root"
HOME="$test_root/home" \
XDG_CONFIG_HOME="$test_root/home/.config" \
FAKE_SYSTEMCTL_LOG="$systemctl_log" \
PATH="$fake_bin:$PATH" \
  "$project_root/install.sh"
)

test -x "$test_root/home/.local/bin/ryoku-palette-bridge"
test -x "$test_root/home/.local/bin/ryoku-palette-bridge-doctor"
test -x "$test_root/home/.local/bin/ryoku-palette-bridge-remove-integrations"
cmp "$project_root/doctor.sh" "$test_root/home/.local/bin/ryoku-palette-bridge-doctor"
cmp "$project_root/packaging/systemd/ryoku-palette-bridge.service" \
  "$test_root/home/.config/systemd/user/ryoku-palette-bridge.service"
grep -Fxq -- '--user daemon-reload' "$systemctl_log"
grep -Fxq -- '--user disable --now ryoku-spicetify-palette.service' "$systemctl_log"
grep -Fxq -- '--user disable --now spiceflow.service' "$systemctl_log"
! grep -Fq -- '--user enable --now ryoku-palette-bridge.service' "$systemctl_log"

cmp "$project_root/vesktop/quickcss.sh" \
  "$test_root/home/.local/share/ryoku/palette-bridge/vesktop/quickcss.sh"
# Invoke the copied executable outside the checkout to exercise helper lookup.
mkdir -p "$test_root/home/.config/vesktop/settings"
printf '/* user css */\n/* ryoku-palette-bridge:begin */\n:root { --ryo-bridge-enabled: 1; }\n/* ryoku-palette-bridge:end */\n' \
  > "$test_root/home/.config/vesktop/settings/quickCss.css"
(cd "$test_root"
HOME="$test_root/home" XDG_CONFIG_HOME="$test_root/home/.config" \
XDG_STATE_HOME="$test_root/home/.local/state" PATH="$fake_bin:$PATH" \
  "$test_root/home/.local/bin/ryoku-palette-bridge-remove-integrations" --vesktop
)
grep -Fxq '/* user css */' "$test_root/home/.config/vesktop/settings/quickCss.css"
! grep -Fq -- '--ryo-bridge-enabled' "$test_root/home/.config/vesktop/settings/quickCss.css"

printf 'PASS: core installer migrates legacy services and leaves the canonical unit opt-in\n'
