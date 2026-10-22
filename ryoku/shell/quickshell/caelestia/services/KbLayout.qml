pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property list<var> layouts: []
    property int activeIndex: -1
    property bool started: false

    readonly property string activeLabel: activeIndex >= 0 && activeIndex < layouts.length ? layouts[activeIndex].label : ""
    readonly property string activeToken: activeIndex >= 0 && activeIndex < layouts.length ? layouts[activeIndex].token : ""
    readonly property string activeShortLabel: shortLabel(activeLabel, activeToken)

    function start(): void {
        if (started)
            return;

        started = true;
        layoutsProc.running = true;
    }

    function refresh(): void {
        layoutsProc.running = true;
    }

    function switchTo(index: int): void {
        switchProc.command = ["qdbus6", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.setLayout", String(index)];
        switchProc.running = true;
    }

    function shortLabel(label: string, token: string): string {
        const normalizedToken = (token || "").trim();
        if (/^[a-z]{2,3}$/i.test(normalizedToken))
            return normalizedToken.toLowerCase();

        const source = (label || normalizedToken).replace(/\s*\([^)]*\)/, "").replace(/[^a-z]/gi, "");
        return source.slice(0, 3).toLowerCase();
    }

    function parseLayouts(output: string): void {
        const parsed = [];
        const re = /\[Argument: \(sss\) "([^"]*)", "([^"]*)", "([^"]*)"\]/g;
        let match;
        while ((match = re.exec(output)) !== null) {
            parsed.push({
                name: match[1],
                label: match[2] || match[1],
                token: match[3] || match[1]
            });
        }

        if (parsed.length > 0)
            layouts = parsed;
        activeProc.running = true;
    }

    function parseActiveIndex(output: string): void {
        const index = parseInt(output.trim(), 10);
        activeIndex = index >= 0 && index < layouts.length ? index : -1;
    }

    Component.onCompleted: start()

    Process {
        id: layoutsProc

        command: ["qdbus6", "--literal", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.getLayoutsList"]
        stdout: StdioCollector {
            onStreamFinished: root.parseLayouts(text)
        }
    }

    Process {
        id: activeProc

        command: ["qdbus6", "org.kde.keyboard", "/Layouts", "org.kde.KeyboardLayouts.getLayout"]
        stdout: StdioCollector {
            onStreamFinished: root.parseActiveIndex(text)
        }
    }

    Process {
        id: switchProc

        onRunningChanged: if (!running)
            activeProc.running = true
    }

    Process {
        id: dbusMonitor

        command: ["dbus-monitor", "--session", "type='signal',interface='org.kde.KeyboardLayouts'"]
        running: root.started

        stdout: SplitParser {
            onRead: text => {
                if (text.indexOf("layoutChanged") !== -1) {
                    if (!activeProc.running)
                        activeProc.running = true
                } else if (text.indexOf("layoutListChanged") !== -1) {
                    if (!layoutsProc.running)
                        layoutsProc.running = true
                }
            }
        }
    }
}
