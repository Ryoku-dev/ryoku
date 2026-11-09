// ps rows for the Game Bar tasks card. The command is
// `ps -eo user:64=,pid=,comm=,cputimes=,etimes=,rss=`: user is the one leading
// token, and its column is widened because the default user= truncates a long
// name and appends `+`, which would silently drop those rows from the current
// user's filter. comm may contain spaces, so the numeric columns are peeled off
// both ends and whatever remains is the name. cputimes is CPU time in seconds over
// the process lifetime, which is the average load, not an instantaneous
// sample: good enough for a short list.

const PROTECTED = [
    "systemd", "systemd-logind", "logind", "dbus-daemon", "dbus-broker",
    "niri", "hyprland", "xwayland", "xorg", "qs", "quickshell",
    "ryoku-shell", "ryoku-wm", "ryoku-wm-niri", "ryoku-host",
    "pipewire", "pipewire-pulse", "wireplumber", "gnome-keyring",
    "polkitd", "swaybg", "mpvpaper", "prowl", "gpu-screen-rec"
];

// `started` is the process start as an absolute epoch second, derived from the
// scan time and etimes. A pid alone does not identify a process: Linux reuses
// pids, so a row must carry this to stay armed against the same process.
function parse(text, scannedAtMs) {
    const scannedAt = typeof scannedAtMs === "number" ? scannedAtMs : Date.now();
    const out = [];
    const rows = String(text || "").split("\n");
    for (let i = 0; i < rows.length; i++) {
        const line = rows[i].trim();
        if (line.length === 0)
            continue;
        const parts = line.split(/\s+/);
        if (parts.length < 6)
            continue;
        const user = parts[0];
        const pid = parseInt(parts[1], 10);
        const rssKb = parseInt(parts[parts.length - 1], 10);
        const secs = parseInt(parts[parts.length - 2], 10);
        const cpuSecs = parseInt(parts[parts.length - 3], 10);
        const name = parts.slice(2, parts.length - 3).join(" ");
        if (!(pid > 0) || name.length === 0)
            continue;
        const cpu = Math.max(0, cpuSecs / Math.max(1, secs) * 100);
        out.push({
            pid: pid,
            name: name,
            user: user,
            cpu: cpu,
            memMiB: (rssKb || 0) / 1024,
            secs: secs || 0,
            started: Math.round((scannedAt - (secs || 0) * 1000) / 1000)
        });
    }
    return out;
}

function isProtected(name) {
    const key = String(name || "").toLowerCase();
    return PROTECTED.indexOf(key) !== -1;
}

function top(text, count, user, scannedAtMs) {
    let rows = parse(text, scannedAtMs);
    if (user)
        rows = rows.filter(row => row.user === user);
    rows.sort((a, b) => b.cpu - a.cpu || b.memMiB - a.memMiB);
    return rows.slice(0, count || 5);
}

if (typeof module !== "undefined" && module.exports)
    module.exports = { parse, isProtected, top, PROTECTED };
