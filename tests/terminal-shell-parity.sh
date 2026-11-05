#!/usr/bin/env bash
set -euo pipefail

repo=${RYOKU_PATH:-$(cd "$(dirname "$0")/.." && pwd)}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

fail() { printf 'terminal-shell-parity: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null || fail "$1 is not installed"; }

if grep -RqsE '^[[:space:]]*shell[[:space:]]+/usr/bin/(fish|bash|zsh)([[:space:]]|$)' "$repo/ryoku/apps"; then
  fail "a terminal emulator pins a shell instead of following the account choice"
fi
grep -Fq 'apps/kitty/kitty.conf' "$repo/ryoku/shell/deploy.sh" ||
  fail "dev deploy does not refresh Kitty's shell policy"
bash_cfg=$repo/ryoku/apps/bash/ryoku.bash
grep -Fq '{ "type": "command", "key": "SHELL"' "$repo/ryoku/apps/fastfetch/config.jsonc" ||
  fail "Fastfetch shell readout does not follow the selected session shell"
grep -Fq 'apps/fastfetch/config.jsonc' "$repo/ryoku/shell/deploy.sh" ||
  fail "dev deploy does not refresh Fastfetch's shell readout"
fzf_line=$(grep -n 'fzf --bash' "$bash_cfg" | sed -n '1s/:.*//p')
ble_line=$(grep -n '/usr/share/blesh/ble.sh' "$bash_cfg" | sed -n '1s/:.*//p')
(( fzf_line < ble_line )) || fail "Bash must initialize fzf before ble.sh"
starship_line=$(grep -n 'starship init bash' "$bash_cfg" | sed -n '1s/:.*//p')
grep -Fxq 'pkgver=0.4.0_devel3' "$repo/release/packages/blesh/PKGBUILD" ||
  fail "ble.sh 0.4 or newer is required for Starship prompt integration"
(( ble_line < starship_line )) || fail "Bash must initialize Starship after ble.sh"
desktop_pkg=$repo/release/packages/ryoku-desktop/PKGBUILD
depends_block=$(awk '/^depends=\(/{d=1;next} d&&/^\)/{exit} d' "$desktop_pkg")
optdepends_block=$(awk '/^optdepends=\(/{d=1;next} d&&/^\)/{exit} d' "$desktop_pkg")
zsh_stack=(zsh zsh-autosuggestions zsh-history-substring-search zsh-syntax-highlighting ryoku-oh-my-zsh)
for pkg in "${zsh_stack[@]}"; do
  if grep -qE "^[[:space:]]*[\"']${pkg}(=[^\"']*)?[\"']" <<<"$depends_block"; then
    fail "$pkg must not be a ryoku-desktop hard dependency"
  fi
  grep -qE "^[[:space:]]*[\"']${pkg}(=[^:]*)?:" <<<"$optdepends_block" ||
    fail "$pkg must be a ryoku-desktop optional dependency"
done
test -f "$repo/release/packages/ryoku-oh-my-zsh/PKGBUILD" ||
  fail "Ryoku Oh My Zsh must remain a signed package"
# shellcheck disable=SC2016  # matching the literal $pkgver text inside the PKGBUILD
grep -Fq 'provides=("oh-my-zsh=$pkgver" "oh-my-zsh-git=$pkgver")' \
  "$repo/release/packages/ryoku-oh-my-zsh/PKGBUILD" ||
  fail "Ryoku Oh My Zsh must version-provide both replacement package names"
for field in conflicts replaces; do
  grep -Fq "$field=('oh-my-zsh' 'oh-my-zsh-git')" \
    "$repo/release/packages/ryoku-oh-my-zsh/PKGBUILD" ||
    fail "Ryoku Oh My Zsh must $field both upstream package names"
done
need fish
need bash
need zsh

stub="$tmp/bin"
mkdir -p "$stub"
for tool in ryoku-fastfetch starship zoxide mise fzf fd eza; do
  cat >"$stub/$tool" <<'STUB'
#!/usr/bin/env bash
printf '%s %s\n' "$(basename "$0")" "$*" >>"$RYOKU_SHELL_LOG"
case "$(basename "$0")" in
  starship|zoxide|mise|fzf) printf ':\n' ;;
esac
STUB
  chmod +x "$stub/$tool"
done

write_generated_palette() {
  local shell=$1 home=$2
  case $shell in
    fish)
      mkdir -p "$home/.config/fish/conf.d"
      cat >"$home/.config/fish/conf.d/ryoku-colors.fish" <<'PALETTE'
set -g fish_color_normal 121212
set -g fish_color_command 345678
PALETTE
      ;;
    bash)
      mkdir -p "$home/.config/bash"
      cat >"$home/.config/bash/ryoku-colors.bash" <<'PALETTE'
ble-face -s syntax_default fg=#121212
ble-face -s syntax_command fg=#345678
PALETTE
      ;;
    zsh)
      mkdir -p "$home/.config/zsh"
      cat >"$home/.config/zsh/ryoku-colors.zsh" <<'PALETTE'
if (( ${+parameters[ZSH_HIGHLIGHT_STYLES]} )); then
  ZSH_HIGHLIGHT_STYLES[default]='fg=#121212'
  ZSH_HIGHLIGHT_STYLES[command]='fg=#345678'
fi
PALETTE
      ;;
  esac
}

run_shell() {
  local shell=$1 mode=$2
  local home="$tmp/$shell-$mode" out="$tmp/$shell-$mode.out" log="$tmp/$shell-$mode.log"
  mkdir -p "$home/.config/fish" "$home/.config/bash" "$home/.config/zsh" "$home/.cache" "$home/.local/bin"
  [[ $mode == generated ]] && write_generated_palette "$shell" "$home"
  : >"$log"
  case $shell in
    fish)
      env -u EDITOR -u VISUAL HOME="$home" PATH="$stub:/usr/bin" RYOKU_SHELL_LOG="$log" fish -i -c \
        "source '$repo/ryoku/apps/fish/config.fish'; printf 'ENV=%s|%s|%s|%s\n' \"\$GOBIN\" \"\$CARGO_INSTALL_ROOT\" \"\$EDITOR\" \"\$VISUAL\"; printf 'PALETTE=%s|%s\n' \"\$fish_color_normal\" \"\$fish_color_command\"; functions ls; set -q fish_color_command; and echo EDIT=fish" >"$out" 2>&1 ||
        { cat "$out" >&2; fail "$shell $mode startup failed"; }
      ;;
    bash)
      # shellcheck disable=SC2016
      printf 'printf "ENV=%%s|%%s|%%s|%%s\\\\n" "$GOBIN" "$CARGO_INSTALL_ROOT" "$EDITOR" "$VISUAL"\nalias ls\nble-face --color=never syntax_default syntax_command\nexit\n' |
        script -qec "env -u EDITOR -u VISUAL HOME='$home' XDG_CACHE_HOME='$home/.cache' USER='${USER:-nero}' LANG=C.UTF-8 TERM=xterm-256color PATH='$stub:/usr/bin' RYOKU_SHELL_LOG='$log' bash --noprofile --rcfile '$repo/ryoku/apps/bash/ryoku.bash' -i" /dev/null >"$out" 2>&1 ||
        { cat "$out" >&2; fail "$shell $mode startup failed"; }
      ;;
    zsh)
      env -u EDITOR -u VISUAL HOME="$home" PATH="$stub:/usr/bin" RYOKU_SHELL_LOG="$log" ZDOTDIR="$home/.config/zsh" zsh -dfi -c \
        "source '$repo/ryoku/apps/zsh/ryoku.zsh'; printf 'ENV=%s|%s|%s|%s\\n' \"\$GOBIN\" \"\$CARGO_INSTALL_ROOT\" \"\$EDITOR\" \"\$VISUAL\"; printf 'PALETTE=%s|%s\\n' \"\${ZSH_HIGHLIGHT_STYLES[default]}\" \"\${ZSH_HIGHLIGHT_STYLES[command]}\"; alias ls; (( \$+functions[_zsh_highlight] )) && print EDIT=zsh" >"$out" 2>&1 ||
        { cat "$out" >&2; fail "$shell $mode startup failed"; }
      ;;
  esac
  if grep -Fq 'unsupported readline function' "$out"; then
    fail "$shell $mode startup emitted unsupported Readline warnings"
  fi
  grep -Fq "ENV=$home/.local/bin|$home/.local|nvim|nvim" "$out" || fail "$shell $mode environment mismatch"
  grep -q "eza -lh --group-directories-first" "$out" || fail "$shell $mode ls alias missing"
  test "$(grep -c '^ryoku-fastfetch ' "$log")" -eq 1 || fail "$shell $mode fastfetch count"
  grep -Fq "starship init $shell" "$log" || fail "$shell $mode starship init"
  case $shell in
    fish)
      grep -Fq 'EDIT=fish' "$out" || fail "fish editing layer"
      if [[ $mode == generated ]]; then
        grep -Fq 'PALETTE=121212|345678' "$out" || fail "fish generated palette"
      else
        grep -Fq 'PALETTE=F1F3E4|e2342a' "$out" || fail "fish fallback palette"
      fi
      ;;
    bash)
      grep -Fq 'ble-face syntax_default=fg=#' "$out" || fail "bash editing layer"
      if [[ $mode == generated ]]; then
        grep -Fq 'ble-face syntax_default=fg=#121212' "$out" || fail "bash generated default"
        grep -Fq 'ble-face syntax_command=fg=#345678' "$out" || fail "bash generated command"
      else
        grep -Fiq 'ble-face syntax_default=fg=#F1F3E4' "$out" || fail "bash fallback default"
        grep -Fiq 'ble-face syntax_command=fg=#e2342a' "$out" || fail "bash fallback command"
      fi
      ;;
    zsh)
      grep -Fq 'EDIT=zsh' "$out" || fail "zsh editing layer"
      if [[ $mode == generated ]]; then
        grep -Fq 'PALETTE=fg=#121212|fg=#345678' "$out" || fail "zsh generated palette"
      else
        grep -Fq 'PALETTE=fg=#F1F3E4|fg=#e2342a' "$out" || fail "zsh fallback palette"
      fi
      ;;
  esac
  grep -Fq "zoxide init $shell --cmd cd" "$log" || fail "$shell $mode zoxide init"
  grep -Fq "mise activate $shell" "$log" || fail "$shell $mode mise init"
  grep -Fq "fzf --$shell" "$log" || fail "$shell $mode fzf init"
}

run_zsh_omz() {
  local home="$tmp/zsh-omz" out="$tmp/zsh-omz.out" log="$tmp/zsh-omz.log"
  mkdir -p "$home/.config/zsh" "$home/.local/bin" "$home/oh-my-zsh"
  cat >"$home/oh-my-zsh/oh-my-zsh.sh" <<'OMZ'
typeset -g RYOKU_OMZ_LOADED=1
typeset -g RYOKU_OMZ_THEME="$ZSH_THEME"
typeset -g RYOKU_OMZ_PLUGINS="${plugins[*]}"
OMZ
  : >"$log"
  env HOME="$home" PATH="$stub:/usr/bin" RYOKU_SHELL_LOG="$log" \
    ZDOTDIR="$home/.config/zsh" ZSH="$home/oh-my-zsh" RYOKU_ZSH_PROMPT=oh-my-zsh \
    zsh -dfi -c "source '$repo/ryoku/apps/zsh/ryoku.zsh'; print OMZ=\$RYOKU_OMZ_LOADED:\$RYOKU_OMZ_THEME:\$RYOKU_OMZ_PLUGINS" >"$out" 2>&1 ||
    { cat "$out" >&2; fail "zsh Oh My Zsh startup failed"; }
  grep -Fq 'OMZ=1:robbyrussell:git' "$out" || fail "zsh Oh My Zsh defaults"
  if grep -Fq 'starship init zsh' "$log"; then
    fail "zsh Oh My Zsh prompt mode also initialized Starship"
  fi
}

run_shell fish fallback
run_shell fish generated
run_shell bash fallback
run_shell bash generated
run_shell zsh fallback
run_shell zsh generated
run_zsh_omz
printf 'terminal shell parity: fish bash zsh passed\n'
