#!/usr/bin/env bash
set -u

section() {
  printf '\n== %s ==\n' "$1"
}

section "session environment"
printf 'DISPLAY=%s\n' "${DISPLAY:-}"
printf 'WAYLAND_DISPLAY=%s\n' "${WAYLAND_DISPLAY:-}"
printf 'XDG_SESSION_TYPE=%s\n' "${XDG_SESSION_TYPE:-}"
printf 'DBUS_SESSION_BUS_ADDRESS=%s\n' "${DBUS_SESSION_BUS_ADDRESS:-}"
printf 'XDG_RUNTIME_DIR=%s\n' "${XDG_RUNTIME_DIR:-}"

section "SPICE channel"
if [[ -e /dev/virtio-ports/com.redhat.spice.0 ]]; then
  printf 'channel=present\n'
else
  printf 'channel=missing\n'
fi

section "system services"
if sudo -n true 2>/dev/null; then
  for service in dbus elogind polkitd turnstiled spice-vdagentd; do
    sudo sv status "/var/service/$service" 2>&1 || true
  done
else
  printf 'sudo=unavailable-without-password\n'
  printf 'run: sudo sv status /var/service/{dbus,elogind,polkitd,turnstiled,spice-vdagentd}\n'
fi

section "user services"
if command -v sv >/dev/null 2>&1; then
  for service in dbus inir turnstile-ready; do
    sv status "$HOME/.config/service/$service" 2>&1 || true
  done
else
  printf 'sv=missing\n'
fi

section "SPICE agent"
agent_exit=2
if command -v spice-vdagentd >/dev/null 2>&1; then
  printf 'spice-vdagentd=%s\n' "$(command -v spice-vdagentd)"
else
  printf 'spice-vdagentd=missing\n'
fi
if command -v spice-vdagent >/dev/null 2>&1; then
  printf 'spice-vdagent=%s\n' "$(command -v spice-vdagent)"
  timeout 3s spice-vdagent -d 2>&1
  agent_exit="$?"
  printf 'spice-vdagent-exit=%s\n' "$agent_exit"
else
  printf 'spice-vdagent=missing\n'
fi

section "processes"
pgrep -af '(^|/)(turnstiled|spice-vdagentd|spice-vdagent|runsvdir|runsv|quickshell|qs)([[:space:]]|$)' || true

section "diagnosis"
if [[ ! -e /dev/virtio-ports/com.redhat.spice.0 ]]; then
  printf '%s\n' 'SPICE channel is missing: add a Spice agent channel in Virt-Manager.'
elif ! command -v spice-vdagent >/dev/null 2>&1; then
  printf '%s\n' 'SPICE agent is missing: install the spice-vdagent package.'
elif [[ -z "${DISPLAY:-}" && -n "${WAYLAND_DISPLAY:-}" && "$agent_exit" -ne 0 ]]; then
  printf '%s\n' 'SPICE agent exited in Wayland-only session; this agent requires an X11 DISPLAY.'
elif [[ -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  printf '%s\n' 'No graphical display variable is present in this shell.'
else
  printf '%s\n' 'SPICE channel and graphical environment are present; inspect the agent output above.'
fi
