# Rashin proposes terminal commands on the prompt. Nothing runs until Enter.

if not status is-interactive
    exit
end

if not type -q rashin
    exit
end

function rashin --description 'Ask Rashin from the terminal'
    if test (count $argv) -eq 0
        command rashin
        return $status
    end
    if command rashin __is-command "$argv[1]" >/dev/null 2>&1
        command rashin $argv
        return $status
    end
    set -l payload (RASHIN_LAST_CMD=$__rashin_last_cmd RASHIN_LAST_STATUS=$__rashin_last_status \
        command rashin ask --buffer -- $argv)
    if test -n "$payload"
        commandline -r -- (string join \n -- $payload)
        commandline -f repaint
    end
end

function __rashin_transmute
    set -l buf (commandline)
    if test -z "$buf"
        return
    end
    set -l payload (RASHIN_LAST_CMD=$__rashin_last_cmd RASHIN_LAST_STATUS=$__rashin_last_status \
        command rashin ask --buffer -- $buf 2>/dev/null)
    if test -n "$payload"
        commandline -r -- (string join \n -- $payload)
    end
    commandline -f repaint
end

function __rashin_bindings
    bind alt-r __rashin_transmute 2>/dev/null; or bind \er __rashin_transmute
    if bind -M insert >/dev/null 2>&1
        bind -M insert alt-r __rashin_transmute 2>/dev/null; or bind -M insert \er __rashin_transmute
    end
end
__rashin_bindings

function __rashin_context --on-event fish_postexec
    set -g __rashin_last_status $status
    set -g __rashin_last_cmd $argv
end
