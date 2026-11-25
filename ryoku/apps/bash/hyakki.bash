command -v hyakki >/dev/null 2>&1 || return

__hyakki_apply_buffer() {
  local payload=$1
  if declare -F ble-edit/content/reset >/dev/null; then
    ble-edit/content/reset "$payload"
  else
    READLINE_LINE=$payload
    READLINE_POINT=${#READLINE_LINE}
  fi
}

hyakki() {
  if (($# == 0)); then
    command hyakki
    return
  fi
  if command hyakki __is-command "${1:-}" >/dev/null 2>&1; then
    command hyakki "$@"
    return
  fi
  local payload
  payload=$(HYAKKI_LAST_CMD=${__hyakki_last_cmd:-} HYAKKI_LAST_STATUS=${__hyakki_last_status:-0} \
    command hyakki ask --buffer -- "$@") || return
  [[ -n $payload ]] && __hyakki_apply_buffer "$payload"
}

__hyakki_transmute() {
  [[ -n ${READLINE_LINE:-} ]] || return
  local payload
  payload=$(HYAKKI_LAST_CMD=${__hyakki_last_cmd:-} HYAKKI_LAST_STATUS=${__hyakki_last_status:-0} \
    command hyakki ask --buffer -- "$READLINE_LINE" 2>/dev/null) || return
  [[ -n $payload ]] && __hyakki_apply_buffer "$payload"
}
bind -x '"\er":__hyakki_transmute'

__hyakki_context() {
  local st=$? ran
  ran=$(HISTTIMEFORMAT='' history 1)
  ran=${ran#*[0-9]  }
  __hyakki_last_cmd=$ran
  __hyakki_last_status=$st
}

if declare -p PROMPT_COMMAND 2>/dev/null | grep -q 'declare -a'; then
  PROMPT_COMMAND+=(__hyakki_context)
elif [[ -n ${PROMPT_COMMAND:-} ]]; then
  PROMPT_COMMAND="__hyakki_context; $PROMPT_COMMAND"
else
  PROMPT_COMMAND=__hyakki_context
fi
