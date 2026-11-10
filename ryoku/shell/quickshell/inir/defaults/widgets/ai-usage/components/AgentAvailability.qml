pragma ComponentBehavior: Bound

// Which agents this machine has actually used. Every collector leaves
// ~/.cache/ai-usage/<tool>-usage.json after collection, so the cache is the
// whole answer: a file means the agent is worth offering in the menu, no file
// means it never ran here.
//
// A FileView on a missing path cannot watch for that path appearing later, so
// the timer reloads all three. `ready` stays false until each FileView has
// reported at least once, so callers never read "no caches yet" as "no agents".
// Publish a complete batch so a newly created OpenCode cache cannot invalidate
// a saved Codex choice before the Codex probe has finished.
import QtQuick
import Quickshell
import Quickshell.Io

Item {
    id: root

    readonly property string cacheDir: Quickshell.env("HOME") + "/.cache/ai-usage"

    property bool _opencode: false
    property bool _claude: false
    property bool _codex: false
    property bool _opencodeSeen: false
    property bool _claudeSeen: false
    property bool _codexSeen: false
    property var _state: ({ opencode: false, claude: false, codex: false, ready: false })
    signal probed

    readonly property bool opencode: root._state.opencode
    readonly property bool claude: root._state.claude
    readonly property bool codex: root._state.codex
    readonly property bool ready: root._state.ready

    Component.onCompleted: root.reloadAll()

    function reloadAll(): void {
        root._opencodeSeen = false
        root._claudeSeen = false
        root._codexSeen = false
        opencodeFile.reload()
        claudeFile.reload()
        codexFile.reload()
    }

    function publish(): void {
        if (!root._opencodeSeen || !root._claudeSeen || !root._codexSeen) return
        root._state = ({ opencode: root._opencode, claude: root._claude,
            codex: root._codex, ready: true })
        root.probed()
    }

    FileView {
        id: opencodeFile
        path: root.cacheDir + "/opencode-usage.json"
        blockLoading: false
        watchChanges: true
        printErrors: false
        onFileChanged: root.reloadAll()
        onLoaded: {
            root._opencode = true
            root._opencodeSeen = true
            root.publish()
        }
        onLoadFailed: {
            root._opencode = false
            root._opencodeSeen = true
            root.publish()
        }
    }

    FileView {
        id: claudeFile
        path: root.cacheDir + "/claude-usage.json"
        blockLoading: false
        watchChanges: true
        printErrors: false
        onFileChanged: root.reloadAll()
        onLoaded: {
            root._claude = true
            root._claudeSeen = true
            root.publish()
        }
        onLoadFailed: {
            root._claude = false
            root._claudeSeen = true
            root.publish()
        }
    }

    FileView {
        id: codexFile
        path: root.cacheDir + "/codex-usage.json"
        blockLoading: false
        watchChanges: true
        printErrors: false
        onFileChanged: root.reloadAll()
        onLoaded: {
            root._codex = true
            root._codexSeen = true
            root.publish()
        }
        onLoadFailed: {
            root._codex = false
            root._codexSeen = true
            root.publish()
        }
    }

    // A FileView normally reports a missing file, but do not let one silent
    // probe keep the menu disabled forever: after a moment, accept what has
    // been seen and let the periodic reload fill in the rest.
    Timer {
        interval: 1200
        repeat: false
        running: true
        onTriggered: {
            root._opencodeSeen = true
            root._claudeSeen = true
            root._codexSeen = true
            root.publish()
        }
    }
    Timer {
        interval: 30000
        repeat: true
        triggeredOnStart: true
        running: true
        onTriggered: root.reloadAll()
    }
}
