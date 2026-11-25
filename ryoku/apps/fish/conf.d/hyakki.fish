# Hyakki proposes terminal commands on the prompt. Nothing runs until Enter.

if not status is-interactive
    exit
end

if not type -q hyakki
    exit
end

function hyakki --description 'Ask Hyakki from the terminal'
    if test (count $argv) -eq 0
        command hyakki
        return $status
    end
    if command hyakki __is-command "$argv[1]" >/dev/null 2>&1
        command hyakki $argv
        return $status
    end
    set -l payload (HYAKKI_LAST_CMD=$__hyakki_last_cmd HYAKKI_LAST_STATUS=$__hyakki_last_status \
        command hyakki ask --buffer -- $argv)
    if test -n "$payload"
        commandline -r -- (string join \n -- $payload)
        commandline -f repaint
    end
end

function __hyakki_transmute
    set -l buf (commandline)
    if test -z "$buf"
        return
    end
    set -l payload (HYAKKI_LAST_CMD=$__hyakki_last_cmd HYAKKI_LAST_STATUS=$__hyakki_last_status \
        command hyakki ask --buffer -- $buf 2>/dev/null)
    if test -n "$payload"
        commandline -r -- (string join \n -- $payload)
    end
    commandline -f repaint
end

function __hyakki_bindings
    bind alt-r __hyakki_transmute 2>/dev/null; or bind \er __hyakki_transmute
    if bind -M insert >/dev/null 2>&1
        bind -M insert alt-r __hyakki_transmute 2>/dev/null; or bind -M insert \er __hyakki_transmute
    end
end
__hyakki_bindings

function __hyakki_context --on-event fish_postexec
    set -g __hyakki_last_status $status
    set -g __hyakki_last_cmd $argv
end
