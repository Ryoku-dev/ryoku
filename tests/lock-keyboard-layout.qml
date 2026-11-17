import QtQuick
import QtQuick.Window
import Quickshell
import Ryoku.Ui.Singletons
ShellRoot {
    id: root
    property int phase: 0
    function fail(msg) { console.error("FAIL: " + msg); Qt.quit() }
    LayoutFeed { id: feed }
    Window {
        id: win
        visible: true
        width: 800; height: 600
        LayoutBadge {
            id: badge
            feed: feed
            shortcut: "Ctrl+Alt+Space"
        }
    }
    Timer {
        interval: 200; running: true; repeat: true
        onTriggered: {
            if (root.phase === 0 && feed.active === "English (US)") {
                if (!badge.visible) return root.fail("badge hidden while the compositor can switch layouts")
                feed.cycle()
                root.phase = 1
            } else if (root.phase === 1 && feed.active === "Russian") {
                // the daemon replays an older frame after a reconnect
                root.phase = 2
            } else if (root.phase === 2 && !feed.switchable) {
                if (feed.active !== "Russian") return root.fail("a stale frame replaced the active layout: " + feed.active)
                if (badge.visible) return root.fail("badge shown on a compositor that cannot switch layouts")
                console.log("PASS: lock layout badge follows the daemon and cycles through the provider")
                Qt.quit()
            }
        }
    }
    Timer { interval: 12000; running: true; onTriggered: root.fail("timed out in phase " + root.phase + " active=" + feed.active) }
}
