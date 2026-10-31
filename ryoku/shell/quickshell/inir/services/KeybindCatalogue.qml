pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property var categories: []
    property bool loaded: false
    property string error: ""

    readonly property var substitutions: ({
        "SUPER": "Super", "CTRL": "Ctrl", "ALT": "Alt", "SHIFT": "Shift",
        "RETURN": "Enter", "ESCAPE": "Esc", "BACKSPACE": "⌫", "PRINT": "PrtSc",
        "LEFT": "←", "RIGHT": "→", "UP": "↑", "DOWN": "↓",
        "PAGE_UP": "PgUp", "PAGE_DOWN": "PgDn", "WHEELSCROLLUP": "Scroll ↑",
        "WHEELSCROLLDOWN": "Scroll ↓", "WHEELSCROLLLEFT": "Scroll ←",
        "WHEELSCROLLRIGHT": "Scroll →"
    })

    function keyLabel(value): string {
        const key = String(value ?? "")
        return root.substitutions[key.toUpperCase()] ?? key
    }

    function keysOf(bind): var {
        const keys = Array.from(bind?.keys ?? []).filter(key => String(key).length > 0)
        if (keys.length > 0)
            return keys.map(root.keyLabel)
        return String(bind?.chord ?? bind?.combo ?? "")
            .split(/\s*\+\s*/).filter(key => key.length > 0).map(root.keyLabel)
    }

    function reload(): void {
        root.loaded = false
        root.error = ""
        catalogue.running = false
        catalogue.running = true
    }

    Process {
        id: catalogue
        running: true
        command: ["ryoku-hub", "keybinds"]
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const value = JSON.parse(this.text)
                    root.categories = Array.from(value?.categories ?? []).map(category => ({
                        name: String(category?.name ?? ""),
                        binds: Array.from(category?.binds ?? []).filter(bind => bind?.disabled !== true)
                    })).filter(category => category.binds.length > 0)
                } catch (error) {
                    root.categories = []
                    root.error = String(error)
                }
                root.loaded = true
            }
        }
        stderr: StdioCollector {}
        onExited: (code, status) => {
            if (code !== 0 && root.error.length === 0) {
                root.categories = []
                root.error = "ryoku-hub keybinds exited " + code
                root.loaded = true
            }
        }
    }
}
