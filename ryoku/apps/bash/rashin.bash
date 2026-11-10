command -v rashin >/dev/null 2>&1 || return

__rashin_apply_buffer() {
  local payload=$1
  if declare -F ble-edit/content/reset >/dev/null; then
    ble-edit/content/reset "$payload"
  else
    READLINE_LINE=$payload
    READLINE_POINT=${#READLINE_LINE}
  fi
}

rashin() {
  if (($# == 0)); then
    command rashin
    return
  fi
  if command rashin __is-command "${1:-}" >/dev/null 2>&1; then
    command rashin "$@"
    return
  fi
  local payload
  payload=$(RASHIN_LAST_CMD=${__rashin_last_cmd:-} RASHIN_LAST_STATUS=${__rashin_last_status:-0} \
    command rashin ask --buffer -- "$@") || return
  [[ -n $payload ]] && __rashin_apply_buffer "$payload"
}

__rashin_transmute() {
  [[ -n ${READLINE_LINE:-} ]] || return
  local payload
  payload=$(RASHIN_LAST_CMD=${__rashin_last_cmd:-} RASHIN_LAST_STATUS=${__rashin_last_status:-0} \
    command rashin ask --buffer -- "$READLINE_LINE" 2>/dev/null) || return
  [[ -n $payload ]] && __rashin_apply_buffer "$payload"
}
bind -x '"\er":__rashin_transmute'

__rashin_context() {
  local st=$? ran
  ran=$(HISTTIMEFORMAT='' history 1)
  ran=${ran#*[0-9]  }
  __rashin_last_cmd=$ran
  __rashin_last_status=$st
}

if declare -p PROMPT_COMMAND 2>/dev/null | grep -q 'declare -a'; then
  PROMPT_COMMAND+=(__rashin_context)
elif [[ -n ${PROMPT_COMMAND:-} ]]; then
  PROMPT_COMMAND="__rashin_context; $PROMPT_COMMAND"
else
  PROMPT_COMMAND=__rashin_context
fi
