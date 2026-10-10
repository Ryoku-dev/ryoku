pragma Singleton
pragma ComponentBehavior: Bound

import inir.modules.common
import inir.modules.common.functions
import inir.services
import QtQuick
import Quickshell
import Quickshell.Io

/**
 * PackageSearch — Async package manager search service.
 *
 * Searches pacman/AUR, XBPS, or DNF repositories for packages matching a query.
 * Results are parsed into structured objects with name, version, repo,
 * description, and installed status.
 *
 * Usage:
 *   PackageSearch.search("vesktop")
 *   // results available in PackageSearch.results after search completes
 */
Singleton {
    id: root

    property string query: ""
    property bool searching: false
    property var results: []
    property string error: ""

    // Debounce to avoid spamming package manager
    property int debounceMs: 300

    function _safeTerminal(): string {
        const configured = (Config.options?.apps?.terminal ?? "kitty").trim()
        if (configured.length === 0)
            return "kitty"
        if (!/^[A-Za-z0-9._+-]+$/.test(configured))
            return "kitty"
        return configured
    }

    function _runTerminalScript(script: string, args): void {
        const command = ["/usr/bin/bash", "-lc", script + "\nprintf \"\\nPress Enter to close...\"\nread", "bash", ...(args ?? [])]
        const terminal = root._safeTerminal()
        if (terminal === "wezterm") {
            ShellExec.execDetachedArgs([terminal, "start", "--always-new-process", "--", ...command], "Run package action")
            return
        }
        ShellExec.execDetachedArgs([terminal, "-e", ...command], "Run package action")
    }

    function isSafePackageName(name: string): bool {
        const pkg = (name ?? "").trim()
        return pkg.length > 0 && /^[A-Za-z0-9@._+-]+$/.test(pkg)
    }

    function installPackage(name: string, preferAurHelper: bool): bool {
        const pkg = (name ?? "").trim()
        if (!root.isSafePackageName(pkg)) {
            Quickshell.execDetached(["/usr/bin/notify-send", Translation.tr("Install Package"),
                Translation.tr("Invalid package name"), "-a", "Shell"])
            return false
        }

        const script = preferAurHelper
            ? "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
                "pacman) if command -v yay &>/dev/null; then yay -S -- \"$1\"; " +
                    "elif command -v paru &>/dev/null; then paru -S -- \"$1\"; else sudo pacman -S -- \"$1\"; fi ;; " +
                "xbps) sudo xbps-install -S -- \"$1\" ;; " +
                "dnf) printf 'The AUR is not available on Fedora\\n' >&2; exit 5 ;; " +
                "*) printf 'No supported package manager found\\n' >&2; exit 127 ;; esac"
            : "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
                "pacman) sudo pacman -S -- \"$1\" ;; " +
                "xbps) sudo xbps-install -S -- \"$1\" ;; " +
                "dnf) sudo ryoku-host pkg install \"$1\" ;; " +
                "*) printf 'No supported package manager found\\n' >&2; exit 127 ;; esac"
        root._runTerminalScript(script, [pkg])
        return true
    }

    function removePackage(name: string): bool {
        const pkg = (name ?? "").trim()
        if (!root.isSafePackageName(pkg)) {
            Quickshell.execDetached(["/usr/bin/notify-send", Translation.tr("Remove Package"),
                Translation.tr("Invalid package name"), "-a", "Shell"])
            return false
        }

        root._runTerminalScript(
            "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
            "pacman) sudo pacman -Rns -- \"$1\" ;; " +
            "xbps) sudo xbps-remove -R -- \"$1\" ;; " +
            "dnf) sudo ryoku-host pkg remove \"$1\" ;; " +
            "*) printf 'No supported package manager found\\n' >&2; exit 127 ;; esac",
            [pkg]
        )
        return true
    }

    function updateSystem(): void {
        root._runTerminalScript(
            "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
            "pacman) if command -v yay &>/dev/null; then yay; elif command -v paru &>/dev/null; then paru; else sudo pacman -Syu; fi ;; " +
            "xbps) sudo xbps-install -Su ;; " +
            "dnf) ryoku update --system ;; " +
            "*) printf 'No supported package manager found\\n' >&2; exit 127 ;; esac",
            []
        )
    }

    // apps.update is a person's own command. Keep custom commands intact; the
    // former stage default is made host-aware so existing Fedora configs never
    // try to launch pacman.
    function runConfiguredUpdate(): void {
        const cmd = (Config.options?.apps?.update ?? "").trim()
        const oldStageDefault = "kitty -1 --hold=yes fish -i -c 'pkexec pacman -Syu'"
        if (cmd === oldStageDefault) {
            ShellExec.execCmd("if [ \"$(ryoku-host pkgmgr 2>/dev/null)\" = dnf ]; then " +
                "kitty -1 --hold=yes fish -i -c 'ryoku update --system'; else exec " + cmd + "; fi")
            return
        }
        const legacyDefault = cmd === "kitty -e arch-update" || cmd === "kitty -e sudo pacman -Syu"
        if (cmd.length === 0 || legacyDefault) {
            root.updateSystem()
            return
        }
        ShellExec.execCmd(cmd)
    }

    function cleanPackageCache(): void {
        root._runTerminalScript(
            "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
            "pacman) if command -v paccache &>/dev/null; then sudo paccache -rk1; else printf 'paccache is unavailable; install pacman-contrib\\n' >&2; exit 127; fi ;; " +
            "xbps) sudo xbps-remove -O ;; " +
            "dnf) dnf_cmd=$(command -v dnf5 || command -v dnf) || exit 127; sudo \"$dnf_cmd\" clean packages ;; " +
            "*) printf 'No supported package cache cleaner found\\n' >&2; exit 127 ;; esac",
            []
        )
    }

    function _xbpsSearchPipeline(mode: string, limit: int): string {
        return "xbps-query " + mode + " \"$1\" 2>/dev/null | head -" + limit + " | " +
            "while IFS= read -r line; do " +
            "status=${line:1:1}; rest=${line#*] }; pkgver=${rest%% *}; " +
            "name=$(xbps-uhelper getpkgname \"$pkgver\"); version=${pkgver#\"$name\"-}; " +
            "desc=${rest#\"$pkgver\"}; desc=\"${desc#\"${desc%%[![:space:]]*}\"}\"; " +
            "printf '__XBPS__\\t%s\\t%s\\t%s\\t%s\\n' \"$status\" \"$name\" \"$version\" \"$desc\"; " +
            "done"
    }

    function _dnfSearchPipeline(installedOnly: bool, limit: int): string {
        const source = installedOnly
            ? "rpm -qa --qf '%{NAME}\\t%{VERSION}-%{RELEASE}\\t@System\\t%{SUMMARY}\\n' | grep -iF -- \"$1\""
            : "dnf_cmd=$(command -v dnf5 || command -v dnf) || exit 127; " +
                "\"$dnf_cmd\" -q repoquery --available --qf '%{name}\\t%{evr}\\t%{repoid}\\t%{summary}\\n' \"*$1*\""
        return source + " | sed '/^$/d' | head -" + limit + " | while IFS=$'\\t' read -r name version repo desc; do [ -n \"$name\" ] || continue; " +
            "installed=0; rpm -q --quiet \"$name\" && installed=1; " +
            "printf '__DNF__\\t%s\\t%s\\t%s\\t%s\\t%s\\n' \"$installed\" \"$name\" \"$version\" \"$repo\" \"$desc\"; " +
            "done"
    }

    function search(q: string): void {
        root.query = q.trim()
        if (root.query === "") {
            root.results = []
            root.searching = false
            root.error = ""
            _debounceTimer.stop()
            return
        }
        _debounceTimer.restart()
    }

    function clear(): void {
        root.query = ""
        root.results = []
        root.searching = false
        root.error = ""
        _debounceTimer.stop()
        _searchProc.running = false
    }

    Timer {
        id: _debounceTimer
        interval: root.debounceMs
        onTriggered: {
            if (root.query === "") return
            root.searching = true
            root.error = ""
            _stdout = ""
            const xbpsSearch = root._xbpsSearchPipeline("-Rs", 200)
            const dnfSearch = root._dnfSearchPipeline(false, 200)
            _searchProc.command = ["/usr/bin/bash", "-lc",
                "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
                "pacman) if command -v yay &>/dev/null; then yay -Ss \"$1\" 2>/dev/null | head -200; " +
                    "elif command -v paru &>/dev/null; then paru -Ss \"$1\" 2>/dev/null | head -200; " +
                    "else pacman -Ss \"$1\" 2>/dev/null | head -200; fi ;; " +
                "xbps) " + xbpsSearch + " ;; " +
                "dnf) " + dnfSearch + " ;; " +
                "*) exit 127 ;; esac",
                "bash", root.query
            ]
            _searchProc.running = true
        }
    }

    property string _stdout: ""

    Process {
        id: _searchProc
        stdout: SplitParser {
            splitMarker: ""
            onRead: data => { root._stdout += data }
        }
        onExited: (exitCode, exitStatus) => {
            root.searching = false
            if (exitCode !== 0 && root._stdout.trim() === "") {
                root.results = []
                return
            }
            root.results = root._parseResults(root._stdout)
        }
    }

    // Timeout for slow searches
    Timer {
        id: _timeoutTimer
        interval: 15000
        running: _searchProc.running
        onTriggered: {
            _searchProc.running = false
            root.searching = false
            root.error = "Search timed out"
        }
    }

    function _parseResults(output: string): list<var> {
        const lines = output.split("\n")
        const pkgs = []
        let i = 0
        while (i < lines.length) {
            const line = lines[i]
            if (line.startsWith("__XBPS__\t")) {
                const fields = line.split("\t")
                if (fields.length >= 5) {
                    pkgs.push({
                        name: fields[2],
                        version: fields[3],
                        repo: "xbps",
                        description: fields.slice(4).join("\t"),
                        installed: fields[1] === "*",
                        votes: 0,
                        popularity: 0,
                        isAur: false
                    })
                }
                i++
                continue
            }
            if (line.startsWith("__DNF__\t")) {
                const fields = line.split("\t")
                if (fields.length >= 6) {
                    pkgs.push({
                        name: fields[2],
                        version: fields[3],
                        repo: fields[4],
                        description: fields.slice(5).join("\t"),
                        installed: fields[1] === "1",
                        votes: 0,
                        popularity: 0,
                        isAur: false
                    })
                }
                i++
                continue
            }


            // Package line format: "repo/name version [size] [installed]"
            // or AUR: "aur/name version (+votes popularity) [installed]"
            const pkgMatch = line.match(/^(\S+)\/(\S+)\s+(\S+)\s*(.*)$/)
            if (pkgMatch) {
                const repo = pkgMatch[1]
                const name = pkgMatch[2]
                const version = pkgMatch[3]
                const rest = pkgMatch[4] || ""
                const installed = /\(Installed\)/i.test(rest) || /\[installed\]/i.test(rest) || /\[Installed\]/i.test(rest)

                // Extract AUR popularity/votes if present
                const aurMeta = rest.match(/\(([+-]?\d+)\s+([\d.]+)\)/)
                const votes = aurMeta ? parseInt(aurMeta[1]) : 0
                const popularity = aurMeta ? parseFloat(aurMeta[2]) : 0

                // Next line is description (indented)
                let description = ""
                if (i + 1 < lines.length && lines[i + 1].match(/^\s+/)) {
                    description = lines[i + 1].trim()
                    i++
                }

                pkgs.push({
                    name: name,
                    version: version,
                    repo: repo,
                    description: description,
                    installed: installed,
                    votes: votes,
                    popularity: popularity,
                    isAur: repo === "aur"
                })
            }
            i++
        }
        return pkgs
    }

    // Search for installed packages only (for remove operations)
    function searchInstalled(q: string): void {
        root.query = q.trim()
        if (root.query === "") {
            root.results = []
            root.searching = false
            return
        }
        root.searching = true
        root.error = ""
        _stdout = ""
        const xbpsSearch = root._xbpsSearchPipeline("-s", 100)
        const dnfSearch = root._dnfSearchPipeline(true, 100)
        _installedProc.command = ["/usr/bin/bash", "-lc",
            "manager=$(ryoku-host pkgmgr 2>/dev/null) || exit 127; case \"$manager\" in " +
            "pacman) pacman -Qs \"$1\" 2>/dev/null | head -100 ;; " +
            "xbps) " + xbpsSearch + " ;; " +
            "dnf) " + dnfSearch + " ;; " +
            "*) exit 127 ;; esac",
            "bash", root.query
        ]
        _installedProc.running = true
    }

    Process {
        id: _installedProc
        stdout: SplitParser {
            splitMarker: ""
            onRead: data => { root._stdout += data }
        }
        onExited: (exitCode, exitStatus) => {
            root.searching = false
            if (exitCode !== 0 && root._stdout.trim() === "") {
                root.results = []
                return
            }
            // Parse results and mark all as installed
            const parsed = root._parseResults(root._stdout)
            root.results = parsed.map(pkg => Object.assign({}, pkg, { installed: true }))
        }
    }

    IpcHandler {
        target: "packageSearch"

        function search(query: string): string {
            root.search(query)
            return "searching: " + query
        }

        function results(): string {
            return root.results.map(p =>
                `${p.repo}/${p.name} ${p.version}${p.installed ? " [installed]" : ""}\t${p.description}`
            ).join("\n")
        }
    }
}
