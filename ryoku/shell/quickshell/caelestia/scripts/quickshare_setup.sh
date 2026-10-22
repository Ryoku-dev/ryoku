#!/bin/bash
set -euo pipefail

PORT=""
STATUS_ONLY=0

usage() {
    echo "Usage: quickshare_setup.sh --status [--port PORT]"
    echo "       quickshare_setup.sh --port PORT"
}

avahi_state() {
    if ! command -v avahi-daemon >/dev/null 2>&1; then
        printf 'absent\n'
        return 0
    fi

    if ! command -v systemctl >/dev/null 2>&1; then
        if command -v pgrep >/dev/null 2>&1 && pgrep -x avahi-daemon >/dev/null 2>&1; then
            printf 'active\n'
        else
            printf 'unknown\n'
        fi
        return 0
    fi

    if systemctl is-active --quiet avahi-daemon.service 2>/dev/null; then
        printf 'active\n'
    else
        printf 'inactive\n'
    fi
}

firewalld_in_charge() {
    command -v firewall-cmd >/dev/null 2>&1 || return 1
    command -v systemctl >/dev/null 2>&1 || return 1
    systemctl is-active --quiet firewalld 2>/dev/null
}

ufw_in_charge() {
    command -v ufw >/dev/null 2>&1 || return 1

    if [[ "$EUID" -eq 0 ]]; then
        ufw status 2>/dev/null | grep -qi '^Status: active'
        return
    fi

    command -v systemctl >/dev/null 2>&1 || return 1
    systemctl is-enabled --quiet ufw 2>/dev/null || systemctl is-active --quiet ufw 2>/dev/null
}

firewall_backend() {
    if firewalld_in_charge; then
        printf 'firewalld\n'
    elif ufw_in_charge; then
        printf 'ufw\n'
    else
        printf 'none\n'
    fi
}

port_state() {
    local backend="$1" port="$2" answer

    case "$backend" in
        firewalld)
            answer="$(firewall-cmd --query-port="$port/tcp" 2>/dev/null || true)"
            case "$answer" in
                yes) printf 'allowed\n' ;;
                no) printf 'blocked\n' ;;
                *) printf 'unknown\n' ;;
            esac
            ;;
        ufw)
            printf 'unknown\n'
            ;;
        *)
            printf 'allowed\n'
            ;;
    esac
}

setup_state() {
    local avahi="$1" backend="$2" port="$3"

    if [[ "$avahi" != "active" ]]; then
        printf 'needed\n'
        return 0
    fi

    case "$backend:$port" in
        none:*) printf 'ok\n' ;;
        firewalld:allowed) printf 'ok\n' ;;
        firewalld:blocked) printf 'needed\n' ;;
        *) printf 'unknown\n' ;;
    esac
}

report_status() {
    local port="$1" avahi backend portstate

    avahi="$(avahi_state)"
    backend="$(firewall_backend)"
    portstate="$(port_state "$backend" "$port")"

    printf 'AVAHI=%s\n' "$avahi"
    printf 'FIREWALL=%s\n' "$backend"
    printf 'PORT=%s\n' "$portstate"
    printf 'SETUP=%s\n' "$(setup_state "$avahi" "$backend" "$portstate")"
}

enable_avahi() {
    if [[ "$(avahi_state)" == "active" ]]; then
        echo "Avahi is already running."
        return 0
    fi

    if ! command -v avahi-daemon >/dev/null 2>&1; then
        echo "The avahi package is not installed. Install it and try again." >&2
        return 1
    fi

    if ! command -v systemctl >/dev/null 2>&1; then
        echo "systemctl is unavailable, so avahi-daemon has to be started by hand." >&2
        return 1
    fi

    echo "Enabling and starting avahi-daemon..."
    systemctl enable --now avahi-daemon.service 2>/dev/null ||
        systemctl enable --now avahi-daemon.socket

    local attempts=0
    while [[ "$(avahi_state)" != "active" && "$attempts" -lt 20 ]]; do
        sleep 0.25
        attempts=$((attempts + 1))
    done
}

open_firewall() {
    local port="$1" backend

    backend="$(firewall_backend)"

    case "$backend" in
        firewalld)
            echo "Allowing port $port/tcp and mDNS through firewalld..."
            firewall-cmd --permanent --add-port="$port/tcp"
            firewall-cmd --permanent --add-service=mdns
            firewall-cmd --reload
            ;;
        ufw)
            echo "Allowing port $port/tcp and mDNS through ufw..."
            ufw allow "$port/tcp"
            ufw allow 5353/udp
            ;;
        *)
            echo "No active firewalld or ufw found; there is no firewall rule to add."
            ;;
    esac
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --status)
            STATUS_ONLY=1
            shift
            ;;
        --port)
            if [[ $# -lt 2 || -z "$2" ]]; then
                echo "--port needs a value" >&2
                usage >&2
                exit 2
            fi
            PORT="$2"
            shift 2
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [[ -z "$PORT" ]]; then
    echo "--port is required: it is the value QuickShareService listens on." >&2
    usage >&2
    exit 2
fi

if ! [[ "$PORT" =~ ^[0-9]+$ ]]; then
    echo "--port must be a TCP port number, not $PORT" >&2
    exit 2
fi

if ((PORT < 1 || PORT > 65535)); then
    echo "--port must be between 1 and 65535, not $PORT" >&2
    exit 2
fi

if [[ "$STATUS_ONLY" -eq 1 ]]; then
    report_status "$PORT"
    exit 0
fi

if [[ "$EUID" -ne 0 ]]; then
    echo "Opening the firewall and enabling Avahi needs root: run this through pkexec or sudo." >&2
    exit 1
fi

enable_avahi
open_firewall "$PORT"
report_status "$PORT"
