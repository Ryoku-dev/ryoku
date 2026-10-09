# shellcheck shell=sh
case ":${PATH:-}:" in
  *":$HOME/.local/bin:"*) ;;
  *) PATH="$HOME/.local/bin${PATH:+:$PATH}" ;;
esac
export PATH
export GOBIN="$HOME/.local/bin"
export CARGO_INSTALL_ROOT="$HOME/.local"
export EDITOR="${EDITOR:-nvim}"
export VISUAL="${VISUAL:-nvim}"

if [ -z "${RYOKU_BASE_OS+x}" ] && command -v ryoku-base-os >/dev/null 2>&1; then
  RYOKU_BASE_OS=$(ryoku-base-os)
  export RYOKU_BASE_OS
fi

if command -v fd >/dev/null 2>&1; then
  export FZF_DEFAULT_COMMAND='fd --hidden --follow --exclude .git'
  export FZF_CTRL_T_COMMAND="$FZF_DEFAULT_COMMAND"
  export FZF_ALT_C_COMMAND='fd --type d --hidden --follow --exclude .git'
fi
