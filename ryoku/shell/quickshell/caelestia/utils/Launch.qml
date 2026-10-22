pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io
import Caelestia.Config

Singleton {
    id: root

    property bool hasSystemdCat: false
    property bool hasApp2Unit: false

    /// Wraps a command so it does not run on the shell's stdio.
    /// Returns the command unchanged when nothing is available to wrap it.
    function wrap(command: list<string>): list<string> {
        if (command.length === 0)
            return command;

        if (root.hasApp2Unit) {
            const appName = command[0].split('/').pop();
            if (root.hasSystemdCat)
                return ["app2unit", "-a", appName, "--", "systemd-cat", "--", ...command];
            return ["app2unit", "-a", appName, "--", ...command];
        }

        if (root.hasSystemdCat)
            return ["systemd-run", "--user", "--scope", "--quiet", "systemd-cat", "--", ...command];

        return ["systemd-run", "--user", "--scope", "--quiet", "--", ...command];
    }

    function exec(command: list<string>): void {
        if (command.length > 0)
            Quickshell.execDetached(root.wrap(command));
    }

    function terminalCommand(command: list<string>): list<string> {
        return [...GlobalConfig.general.apps.terminal, `${Paths.caelestiaDir}/assets/wrap_term_launch.sh`, ...command];
    }

    function launchInTerminal(command: list<string>, workingDirectory: string): void {
        Quickshell.execDetached({
            command: root.wrap(root.terminalCommand(command)),
            workingDirectory: workingDirectory
        });
    }

    function launchEntry(entry: DesktopEntry): void {
        if (entry.runInTerminal)
            root.launchInTerminal(entry.command, entry.workingDirectory);
        else
            Quickshell.execDetached({
                command: root.wrap(entry.command),
                workingDirectory: entry.workingDirectory
            });
    }

    Process {
        running: true
        command: ["sh", "-c", "command -v systemd-cat >/dev/null 2>&1 && echo cat; command -v app2unit >/dev/null 2>&1 && echo unit"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.hasSystemdCat = text.includes("cat");
                root.hasApp2Unit = text.includes("unit");
            }
        }
    }
}
