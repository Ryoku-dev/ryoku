#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bash "$project_root/tests/zen-profiles.test.sh"
test_root=$(mktemp -d "${TMPDIR:-/tmp}/ryoku-palette install's \"quote\".XXXXXX")
trap 'rm -rf "$test_root"' EXIT
config_root="$test_root/config"
data_root="$test_root/data"
fake_bin="$test_root/bin"
profile_root="$config_root/zen/test.default"
state_file="$test_root/spicetify-extensions"
state_root="$test_root/state"
cache_root="$test_root/cache"
export XDG_CACHE_HOME="$cache_root"
# Package checks may run without optional runtime rendering tools installed.
for tool in jq matugen; do
  command -v "$tool" >/dev/null || { printf 'PASS: integration suite skipped (missing %s)\n' "$tool"; exit 0; }
done
mkdir -p "$cache_root/ryoku"
write_carrier() {
  jq -n --arg primary "$1" '{colors: (["inverse_primary", "error", "tertiary_container", "on_primary", "tertiary", "primary", "primary_container", "secondary", "surface_bright", "primary_fixed_dim", "surface", "surface_container", "on_surface", "on_surface_variant", "outline", "outline_variant", "surface_variant", "surface_container_high", "surface_container_low"] | map({key: ., value: {default: {hex: (if . == "primary" then $primary else "#123456" end)}}}) | from_entries)}' > "$cache_root/ryoku/matugen-carrier.json"
}
write_carrier "#abcdef"
mkdir -p "$fake_bin" "$config_root/matugen" "$config_root/vesktop/settings" "$profile_root/chrome"
printf '[config]\n\n[templates.existing]\ninput_path = "keep"\noutput_path = "keep"\n' \
  > "$config_root/matugen/apps.toml"
legacy_overlay="$config_root/ryoku/user_edits/matugen"
mkdir -p "$legacy_overlay/templates"
cp "$config_root/matugen/apps.toml" "$legacy_overlay/apps.toml"
# Upgrading the old direct-output registration removes it from the shipped-roster fork.
printf '\n[templates.vesktop]\ninput_path = "%s"\noutput_path = "%s"\n' \
  "$config_root/matugen/templates/vesktop-colors.css" \
  "$config_root/vesktop/settings/quickCss.css" >> "$legacy_overlay/apps.toml"
cp "$project_root/templates/vesktop-colors.css" "$legacy_overlay/templates/vesktop-colors.css"
printf '{}\n' > "$config_root/vesktop/settings/settings.json"
printf '@import "existing.css";\n' > "$profile_root/chrome/userChrome.css"
printf 'user_pref("existing", true);\n' > "$profile_root/user.js"

printf '%s\n' '#!/usr/bin/env bash' 'exit 0' > "$fake_bin/ryogami"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'if [[ $1 == config && $2 == extensions && $# == 2 ]]; then' \
  '  test -f "$FAKE_SPICETIFY_STATE" && cat "$FAKE_SPICETIFY_STATE"' \
  'elif [[ $1 == config && $2 == extensions ]]; then' \
  '  printf "%s\\n" "$3" >> "$FAKE_SPICETIFY_STATE"' \
  'elif [[ $1 != apply ]]; then' \
  '  exit 2' \
  'fi' > "$fake_bin/spicetify"
chmod +x "$fake_bin/ryogami" "$fake_bin/spicetify"

for _ in 1 2; do
  XDG_CONFIG_HOME="$config_root" \
  XDG_DATA_HOME="$data_root" \
  XDG_STATE_HOME="$state_root" \
  ZEN_PROFILE_ROOT="$profile_root" \
  FAKE_SPICETIFY_STATE="$state_file" \
  PATH="$fake_bin:$PATH" \
    "$project_root/install-integrations.sh" --all
done

bridge="$config_root/ryoku/palette-bridge"
templates="$data_root/ryoku/palette-bridge/templates"
cmp "$project_root/spicetify/ryoku-wallpaper-colors.js" \
  "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js"
cmp "$project_root/templates/vesktop-colors.css" "$templates/vesktop-colors.css"
cmp "$project_root/templates/zen.css" "$templates/zen.css"
cmp "$project_root/vesktop/midnight-ryoku.theme.css" \
  "$config_root/vesktop/themes/midnight-ryoku.theme.css"
[[ $(grep -c '^\[templates\.vesktop\]$' "$bridge/matugen.toml") == 1 ]]
grep -Fxq "output_path = $(jq -Rn --arg path "$state_root/ryoku/palette-bridge/vesktop/palette.css" '$path')" "$bridge/matugen.toml"
grep -q '^post_hook = .*quickcss' "$bridge/matugen.toml"
[[ $(grep -c '^\[templates\.zen\]$' "$bridge/matugen.toml") == 1 ]]
! grep -Fq '[templates.vesktop]' "$legacy_overlay/apps.toml"
[[ $(grep -Fc '@import "ryoku-colors.css";' "$profile_root/chrome/userChrome.css") == 1 ]]
[[ $(grep -Fc 'toolkit.legacyUserProfileCustomizations.stylesheets' "$profile_root/user.js") == 1 ]]
[[ $(grep -Fc 'ryoku-wallpaper-colors.js' "$state_file") == 1 ]]
jq -e '.useQuickCss == true and (.enabledThemes | map(select(. == "midnight-ryoku.theme.css")) | length == 1)' \
  "$config_root/vesktop/settings/settings.json" >/dev/null

for integration in spotify vesktop zen; do
  XDG_CONFIG_HOME="$config_root" \
  XDG_DATA_HOME="$data_root" \
  XDG_STATE_HOME="$state_root" \
  FAKE_SPICETIFY_STATE="$state_file" \
  PATH="$fake_bin:$PATH" \
    "$project_root/remove-integrations.sh" "--$integration"
done

[[ ! -e "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js" ]]
[[ ! -e "$templates/vesktop-colors.css" ]]
[[ ! -e "$templates/zen.css" ]]
[[ ! -e "$config_root/vesktop/themes/midnight-ryoku.theme.css" ]]
! grep -Fq '[templates.vesktop]' "$bridge/matugen.toml"
! grep -Fq '[templates.zen]' "$bridge/matugen.toml"
grep -Fq '[templates.existing]' "$legacy_overlay/apps.toml"
grep -Fq '@import "existing.css";' "$profile_root/chrome/userChrome.css"
! grep -Fq '@import "ryoku-colors.css";' "$profile_root/chrome/userChrome.css"
grep -Fq 'user_pref("existing", true);' "$profile_root/user.js"
! grep -Fq 'toolkit.legacyUserProfileCustomizations.stylesheets' "$profile_root/user.js"
[[ ! -s "$state_root/ryoku/palette-bridge/owned-files.tsv" ]]

# Removing an integration created by the old installer also repairs its
# whole-roster overlay, rather than leaving a dead template in live Matugen.
printf '[config]\n\n[templates.existing]\ninput_path = "keep"\noutput_path = "keep"\n\n[templates.vesktop]\ninput_path = "old"\noutput_path = "old"\n' \
  > "$legacy_overlay/apps.toml"
mkdir -p "$legacy_overlay/templates"
cp "$project_root/templates/vesktop-colors.css" "$legacy_overlay/templates/vesktop-colors.css"
printf 'vesktop\t%s\n' "$legacy_overlay/templates/vesktop-colors.css" \
  > "$state_root/ryoku/palette-bridge/owned-files.tsv"
XDG_CONFIG_HOME="$config_root" XDG_DATA_HOME="$data_root" \
XDG_STATE_HOME="$state_root" PATH="$fake_bin:$PATH" \
  "$project_root/remove-integrations.sh" --vesktop
! grep -Fq '[templates.vesktop]' "$legacy_overlay/apps.toml"
! grep -Fq '[templates.vesktop]' "$config_root/matugen/apps.toml"
grep -Fq '[templates.existing]' "$legacy_overlay/apps.toml"
[[ ! -e "$legacy_overlay/templates/vesktop-colors.css" ]]
[[ ! -s "$state_root/ryoku/palette-bridge/owned-files.tsv" ]]

run_vesktop() {
  XDG_CONFIG_HOME="$config_root" XDG_DATA_HOME="$data_root" \
    XDG_STATE_HOME="$state_root" PATH="$fake_bin:$PATH" \
    "$project_root/$1-integrations.sh" --vesktop
}

# The Vesktop default Ryoku ships keeps its theme and never gains Midnight.
seed="$project_root/../apps/vesktop/settings/settings.json"
cp "$seed" "$config_root/vesktop/settings/settings.json"
run_vesktop install
jq -e --slurpfile seed "$seed" '.useQuickCss == true and .enabledThemes == $seed[0].enabledThemes' \
  "$config_root/vesktop/settings/settings.json" >/dev/null
[[ ! -e "$config_root/vesktop/themes/midnight-ryoku.theme.css" ]]
run_vesktop remove

# The menu must keep Ryoku selected, including when upgrading a Midnight setup.
printf '{"enabledThemes":["custom.theme.css","Ryoku.theme.css","midnight-ryoku.theme.css"]}\n' \
  > "$config_root/vesktop/settings/settings.json"
quick_css="$config_root/vesktop/settings/quickCss.css"
printf '@import url("custom.css");\n:root { --user-before: red; }\n' > "$test_root/custom-before.css"
cp "$test_root/custom-before.css" "$quick_css"
for _ in 1 2; do run_vesktop install; done
jq -e '.useQuickCss == true and .enabledThemes == ["custom.theme.css", "Ryoku.theme.css"]' \
  "$config_root/vesktop/settings/settings.json" >/dev/null
[[ ! -e "$config_root/vesktop/themes/midnight-ryoku.theme.css" ]]
grep -Fq -- '--ryo-bridge-enabled: 1;' "$templates/vesktop-colors.css"

printf ':root { --user-after: gold; }\n' > "$test_root/custom-after.css"
[[ $(head -n 1 "$quick_css") == '@import url("custom.css");' ]]
grep -Fq -- '--ryo-bridge-enabled: 1;' "$quick_css"
grep -Fq -- '--accent-2: #abcdef;' "$quick_css"
[[ $(grep -Fc '/* ryoku-palette-bridge:begin */' "$quick_css") == 1 ]]
cat "$test_root/custom-after.css" >> "$quick_css"
# Exercise the real supplemental wallpaper fan-out, not only the immediate setup merge.
cp "$bridge/matugen.toml" "$test_root/vesktop-render.toml"
matugen json "$cache_root/ryoku/matugen-carrier.json" --config "$test_root/vesktop-render.toml" --quiet
[[ $(head -n 1 "$quick_css") == '@import url("custom.css");' ]] || {
  printf 'FAIL: wallpaper fan-out erased the user QuickCSS import\n' >&2; exit 1;
}
grep -Fq -- '--user-before: red;' "$quick_css"
grep -Fq -- '--user-after: gold;' "$quick_css"
write_carrier "#fedcba"
matugen json "$cache_root/ryoku/matugen-carrier.json" --config "$test_root/vesktop-render.toml" --quiet
grep -Fq -- '--accent-2: #fedcba;' "$quick_css"
! grep -Fq -- '--accent-2: #abcdef;' "$quick_css"
grep -Fq -- '--user-before: red;' "$quick_css"
grep -Fq -- '--user-after: gold;' "$quick_css"
[[ $(grep -Fc '/* ryoku-palette-bridge:begin */' "$quick_css") == 1 ]]
cp "$quick_css" "$test_root/valid-quickcss.css"
printf '/* ryoku-palette-bridge:begin */\n:root { --incomplete-user-block: gold; }\n' > "$quick_css"
cp "$quick_css" "$test_root/incomplete-quickcss.css"
matugen json "$cache_root/ryoku/matugen-carrier.json" --config "$test_root/vesktop-render.toml" --quiet || true
cmp "$test_root/incomplete-quickcss.css" "$quick_css"
cp "$test_root/valid-quickcss.css" "$quick_css"
cat "$test_root/custom-before.css" "$test_root/custom-after.css" > "$test_root/expected.css"
for _ in 1 2; do run_vesktop remove; done
sed '/^[[:space:]]*$/d' "$test_root/expected.css" > "$test_root/expected-noblank.css"
sed '/^[[:space:]]*$/d' "$quick_css" > "$test_root/actual-noblank.css"
cmp "$test_root/expected-noblank.css" "$test_root/actual-noblank.css"
! grep -Fq -- '--ryo-bridge-enabled: 1;' "$quick_css"
jq -e '.useQuickCss == true and .enabledThemes == ["custom.theme.css", "Ryoku.theme.css"]' \
  "$config_root/vesktop/settings/settings.json" >/dev/null
! grep -Fq '[templates.vesktop]' "$bridge/matugen.toml"
[[ ! -e "$state_root/ryoku/palette-bridge/vesktop/quickcss.sh" ]]
[[ ! -e "$state_root/ryoku/palette-bridge/vesktop/palette.css" ]]

# Re-enabling uses the current carrier immediately, preserving custom CSS.
write_carrier "#fedcba"
run_vesktop install
grep -Fq -- '--accent-2: #fedcba;' "$quick_css"
! grep -Fq -- '--accent-2: #abcdef;' "$quick_css"
grep -Fq -- '--user-before: red;' "$quick_css"
grep -Fq -- '--user-after: gold;' "$quick_css"
run_vesktop remove

# Cold setup creates a complete palette even when QuickCSS does not exist.
rm "$quick_css"
run_vesktop install
grep -Fq -- '--accent-2: #fedcba;' "$quick_css"
grep -Fq -- '--bg-4: #123456;' "$quick_css"
! grep -Fq '{{colors.' "$quick_css"
run_vesktop remove

# Preflight failures must not mutate configuration or ownership state.
printf ':root { --keep-on-failure: gold; }\n' > "$quick_css"
cp -a "$config_root" "$test_root/config-before-failure"
cp -a "$state_root" "$test_root/state-before-failure"
assert_failure_unchanged() {
  if run_vesktop install; then
    printf 'setup unexpectedly succeeded\n' >&2
    exit 1
  fi
  diff -r "$test_root/config-before-failure" "$config_root"
  diff -r "$test_root/state-before-failure" "$state_root"
}
rm "$cache_root/ryoku/matugen-carrier.json"
assert_failure_unchanged
printf '{invalid json\n' > "$cache_root/ryoku/matugen-carrier.json"
assert_failure_unchanged
write_carrier "#fedcba"
printf '#!/usr/bin/env bash\nexit 42\n' > "$fake_bin/matugen"
chmod +x "$fake_bin/matugen"
assert_failure_unchanged
rm "$fake_bin/matugen"

# An unmarked palette from the previous release can be safely recognized without
# treating other user :root blocks as generated content.
sed -E '/ryoku-palette-bridge:(begin|end)/d; /--ryo-bridge-enabled:/d; s/The selected theme remains loaded in a stable layer/Midnight remains loaded in a stable theme layer/; s/\{\{[^}]+\}\}/#aBc123/g' \
  "$project_root/templates/vesktop-colors.css" > "$quick_css"
cat "$test_root/custom-after.css" >> "$quick_css"
run_vesktop remove
cmp "$test_root/custom-after.css" "$quick_css"

# Similar-looking but modified legacy content is not ours to delete.
sed -E '/ryoku-palette-bridge:(begin|end)/d; /--ryo-bridge-enabled:/d; s/The selected theme remains loaded in a stable layer/Midnight remains loaded in a stable theme layer/; s/\{\{[^}]+\}\}/#123456/g; s/--spacing: 12px/--spacing: 13px/' \
  "$project_root/templates/vesktop-colors.css" > "$quick_css"
cp "$quick_css" "$test_root/expected.css"
run_vesktop remove
cmp "$test_root/expected.css" "$quick_css"
printf '/* ryoku-palette-bridge:begin */\n:root { --user-color: red; }\n' > "$quick_css"
cp "$quick_css" "$test_root/expected.css"
run_vesktop remove
cmp "$test_root/expected.css" "$quick_css"

# The Flathub build keeps its settings in its own ~/.var/app folder. With no
# native Vesktop, setup and removal follow it there and leave ~/.config alone.
flatpak_home="$test_root/flatpak-home"
flatpak_config="$flatpak_home/.config"
flatpak_vesktop="$flatpak_home/.var/app/dev.vencord.Vesktop/config/vesktop"
mkdir -p "$flatpak_config/matugen" "$flatpak_vesktop/settings"
printf '[config]\n' > "$flatpak_config/matugen/apps.toml"
printf '{}\n' > "$flatpak_vesktop/settings/settings.json"
run_flatpak_vesktop() {
  HOME="$flatpak_home" XDG_CONFIG_HOME="$flatpak_config" XDG_STATE_HOME="$test_root/flatpak-state" \
    PATH="$fake_bin:$PATH" "$project_root/$1-integrations.sh" --vesktop
}
run_flatpak_vesktop install
grep -Fq -- '--accent-2: #fedcba;' "$flatpak_vesktop/settings/quickCss.css"
cmp "$project_root/vesktop/midnight-ryoku.theme.css" "$flatpak_vesktop/themes/midnight-ryoku.theme.css"
jq -e '.useQuickCss == true' "$flatpak_vesktop/settings/settings.json" >/dev/null
grep -Fq 'dev.vencord.Vesktop/config/vesktop/settings/quickCss.css' \
  "$flatpak_config/ryoku/palette-bridge/matugen.toml"
[[ ! -e "$flatpak_config/vesktop" ]]
run_flatpak_vesktop remove
if grep -Fq -- '--ryo-bridge-enabled: 1;' "$flatpak_vesktop/settings/quickCss.css"; then
  printf 'FAIL: removal left the palette in the Flathub QuickCSS\n' >&2; exit 1
fi
[[ ! -e "$flatpak_vesktop/themes/midnight-ryoku.theme.css" ]]
if grep -Fq '[templates.vesktop]' "$flatpak_config/ryoku/palette-bridge/matugen.toml"; then
  printf 'FAIL: removal left the Flathub Vesktop Matugen hook\n' >&2; exit 1
fi

# Ryoku seeds the native Vencord settings on every box, and that seed alone
# does not mean a native Vesktop runs: setup keeps following the Flathub build
# and turns to the native profile once Vesktop has run there.
seeded_vesktop="$flatpak_config/vesktop"
mkdir -p "$seeded_vesktop/settings"
cp "$seed" "$seeded_vesktop/settings/settings.json"
printf '{}\n' > "$flatpak_vesktop/settings/settings.json"
run_flatpak_vesktop install
jq -e '.useQuickCss == true' "$flatpak_vesktop/settings/settings.json" >/dev/null
cmp "$seed" "$seeded_vesktop/settings/settings.json"
[[ ! -e "$seeded_vesktop/settings/quickCss.css" ]]
run_flatpak_vesktop remove
printf '{}\n' > "$seeded_vesktop/state.json"
run_flatpak_vesktop install
jq -e '.useQuickCss == true' "$seeded_vesktop/settings/settings.json" >/dev/null
grep -Fq -- '--accent-2: #fedcba;' "$seeded_vesktop/settings/quickCss.css"
[[ ! -e "$seeded_vesktop/themes/midnight-ryoku.theme.css" ]]
run_flatpak_vesktop remove

# A file the user deleted first, or a removal that stopped halfway, must never
# leave removal stuck: what is gone counts as removed, what will not go stays in
# the ledger for the next try, and the ledger always matches what is left.
stuck="$test_root/stuck"
stuck_ledger="$stuck/state/ryoku/palette-bridge/owned-files.tsv"
gone_profile="$stuck/zen/gone.default"
locked_chrome="$stuck/zen/locked.default/chrome"
mkdir -p "$(dirname "$stuck_ledger")" "$stuck/home" "$locked_chrome"
printf '/* ryoku */\n' > "$locked_chrome/ryoku-colors.css"
other_line=$(printf 'vesktop\t%s' "$stuck/data/ryoku/palette-bridge/templates/vesktop-colors.css")
locked_line=$(printf 'zen\t%s' "$locked_chrome/ryoku-colors.css")
printf 'spotify\t%s\nzen\t%s\nzen\t%s\n%s\n%s\n' \
  "$stuck/config/spicetify/Extensions/ryoku-wallpaper-colors.js" \
  "$gone_profile/chrome/userChrome.css" "$gone_profile/user.js" \
  "$locked_line" "$other_line" > "$stuck_ledger"
run_stuck() {
  HOME="$stuck/home" XDG_CONFIG_HOME="$stuck/config" XDG_DATA_HOME="$stuck/data" \
    XDG_STATE_HOME="$stuck/state" FAKE_SPICETIFY_STATE="$state_file" PATH="$fake_bin:$PATH" \
    "$project_root/remove-integrations.sh" "$1"
}
if ! run_stuck --spotify; then
  printf 'FAIL: removing a Spotify file the user already deleted aborted\n' >&2; exit 1
fi
if (( EUID != 0 )); then
  chmod 0555 "$locked_chrome"
  if run_stuck --zen 2>/dev/null; then
    chmod 0755 "$locked_chrome"
    printf 'FAIL: removal reported success with a Zen file left behind\n' >&2; exit 1
  fi
  chmod 0755 "$locked_chrome"
  if [[ "$(cat "$stuck_ledger")" != "$locked_line"$'\n'"$other_line" ]]; then
    printf 'FAIL: ledger after a partial removal:\n%s\n' "$(cat "$stuck_ledger")" >&2; exit 1
  fi
fi
if ! run_stuck --zen; then
  printf 'FAIL: removing Zen again did not finish what the first try left\n' >&2; exit 1
fi
if [[ -e "$gone_profile" || -e "$locked_chrome/ryoku-colors.css" ]]; then
  printf 'FAIL: Zen removal recreated a deleted profile or left an owned file\n' >&2; exit 1
fi
if [[ "$(cat "$stuck_ledger")" != "$other_line" ]]; then
  printf 'FAIL: ledger after removal:\n%s\n' "$(cat "$stuck_ledger")" >&2; exit 1
fi

printf 'PASS: integration setup and removal are idempotent and preserve existing configuration\n'
