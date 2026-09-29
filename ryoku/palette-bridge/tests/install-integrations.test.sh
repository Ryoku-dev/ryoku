#!/usr/bin/env bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root=$(mktemp -d /tmp/ryoku-palette-install.XXXXXX)
trap 'rm -rf "$test_root"' EXIT
config_root="$test_root/config"
fake_bin="$test_root/bin"
profile_root="$config_root/zen/test.default"
state_file="$test_root/spicetify-extensions"
state_root="$test_root/state"
cache_root="$test_root/cache"
export XDG_CACHE_HOME="$cache_root"
command -v matugen >/dev/null || { printf "matugen is required for integration tests\n" >&2; exit 1; }
mkdir -p "$cache_root/ryoku"
write_carrier() {
  jq -n --arg primary "$1" '{colors: (["inverse_primary", "error", "tertiary_container", "on_primary", "tertiary", "primary", "surface_bright", "primary_fixed_dim", "surface", "on_surface", "on_surface_variant", "outline", "surface_variant", "surface_container_high", "surface_container_low"] | map({key: ., value: {default: {hex: (if . == "primary" then $primary else "#123456" end)}}}) | from_entries)}' > "$cache_root/ryoku/matugen-carrier.json"
}
write_carrier "#abcdef"
mkdir -p "$fake_bin" "$config_root/matugen" "$config_root/vesktop/settings" "$profile_root/chrome"

printf '[config]\n\n[templates.existing]\ninput_path = "keep"\noutput_path = "keep"\n' \
  > "$config_root/matugen/apps.toml"
printf '{}\n' > "$config_root/vesktop/settings/settings.json"
printf '@import "existing.css";\n' > "$profile_root/chrome/userChrome.css"
printf 'user_pref("existing", true);\n' > "$profile_root/user.js"

printf '%s\n' '#!/usr/bin/env bash' 'exit 0' > "$fake_bin/ryoku"
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
chmod +x "$fake_bin/ryoku" "$fake_bin/ryogami" "$fake_bin/spicetify"

for _ in 1 2; do
  XDG_CONFIG_HOME="$config_root" \
  XDG_STATE_HOME="$state_root" \
  ZEN_PROFILE_ROOT="$profile_root" \
  FAKE_SPICETIFY_STATE="$state_file" \
  PATH="$fake_bin:$PATH" \
    "$project_root/install-integrations.sh" --all
done

overlay="$config_root/ryoku/user_edits/matugen"
cmp "$project_root/spicetify/ryoku-wallpaper-colors.js" \
  "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js"
cmp "$project_root/templates/vesktop-colors.css" "$overlay/templates/vesktop-colors.css"
cmp "$project_root/templates/zen.css" "$overlay/templates/zen.css"
cmp "$project_root/vesktop/midnight-ryoku.theme.css" \
  "$config_root/vesktop/themes/midnight-ryoku.theme.css"
[[ $(grep -c '^\[templates\.vesktop\]$' "$overlay/apps.toml") == 1 ]]
[[ $(grep -c '^\[templates\.zen\]$' "$overlay/apps.toml") == 1 ]]
[[ $(grep -c '^\[templates\.existing\]$' "$overlay/apps.toml") == 1 ]]
[[ $(grep -Fc '@import "ryoku-colors.css";' "$profile_root/chrome/userChrome.css") == 1 ]]
[[ $(grep -Fc 'toolkit.legacyUserProfileCustomizations.stylesheets' "$profile_root/user.js") == 1 ]]
[[ $(grep -Fc 'ryoku-wallpaper-colors.js' "$state_file") == 1 ]]
jq -e '.useQuickCss == true and (.enabledThemes | map(select(. == "midnight-ryoku.theme.css")) | length == 1)' \
  "$config_root/vesktop/settings/settings.json" >/dev/null

for integration in spotify vesktop zen; do
  XDG_CONFIG_HOME="$config_root" \
  XDG_STATE_HOME="$state_root" \
  FAKE_SPICETIFY_STATE="$state_file" \
  PATH="$fake_bin:$PATH" \
    "$project_root/remove-integrations.sh" "--$integration"
done

[[ ! -e "$config_root/spicetify/Extensions/ryoku-wallpaper-colors.js" ]]
[[ ! -e "$overlay/templates/vesktop-colors.css" ]]
[[ ! -e "$overlay/templates/zen.css" ]]
[[ ! -e "$config_root/vesktop/themes/midnight-ryoku.theme.css" ]]
! grep -Fq '[templates.vesktop]' "$overlay/apps.toml"
! grep -Fq '[templates.zen]' "$overlay/apps.toml"
grep -Fq '[templates.existing]' "$overlay/apps.toml"
grep -Fq '@import "existing.css";' "$profile_root/chrome/userChrome.css"
! grep -Fq '@import "ryoku-colors.css";' "$profile_root/chrome/userChrome.css"
grep -Fq 'user_pref("existing", true);' "$profile_root/user.js"
! grep -Fq 'toolkit.legacyUserProfileCustomizations.stylesheets' "$profile_root/user.js"
[[ ! -s "$state_root/ryoku/palette-bridge/owned-files.tsv" ]]

run_vesktop() {
  XDG_CONFIG_HOME="$config_root" XDG_STATE_HOME="$state_root" PATH="$fake_bin:$PATH" \
    "$project_root/$1-integrations.sh" --vesktop
}

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
grep -Fq -- '--ryo-bridge-enabled: 1;' "$overlay/templates/vesktop-colors.css"

printf ':root { --user-after: gold; }\n' > "$test_root/custom-after.css"
[[ $(head -n 1 "$quick_css") == '@import url("custom.css");' ]]
grep -Fq -- '--ryo-bridge-enabled: 1;' "$quick_css"
grep -Fq -- '--accent-2: #abcdef;' "$quick_css"
[[ $(grep -Fc '/* ryoku-palette-bridge:begin */' "$quick_css") == 1 ]]
cat "$test_root/custom-after.css" >> "$quick_css"
cat "$test_root/custom-before.css" "$test_root/custom-after.css" > "$test_root/expected.css"
for _ in 1 2; do run_vesktop remove; done
sed '/^[[:space:]]*$/d' "$test_root/expected.css" > "$test_root/expected-noblank.css"
sed '/^[[:space:]]*$/d' "$quick_css" > "$test_root/actual-noblank.css"
cmp "$test_root/expected-noblank.css" "$test_root/actual-noblank.css"
! grep -Fq -- '--ryo-bridge-enabled: 1;' "$quick_css"
jq -e '.useQuickCss == true and .enabledThemes == ["custom.theme.css", "Ryoku.theme.css"]' \
  "$config_root/vesktop/settings/settings.json" >/dev/null
! grep -Fq '[templates.vesktop]' "$overlay/apps.toml"

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

printf 'PASS: integration setup and removal are idempotent and preserve existing configuration\n'
