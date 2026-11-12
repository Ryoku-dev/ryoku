pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// The tour's durations are Tokens' mechanical set verbatim; this singleton only
// adds the reduced-motion gate every sibling surface carries (launcher,
// overview). Reduce motion (shell.json theme.motion.reduce, via Tokens) or
// lowPowerMode collapse every duration to an instant cut so a weak GPU stops
// repainting through transitions.
Singleton {
    readonly property bool reduce: perf.lowPowerMode || Tokens.reduceMotion

    FileView {
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/performance.json"
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        JsonAdapter {
            id: perf
            property bool lowPowerMode: false
        }
    }

    readonly property int snap: reduce ? 0 : Tokens.snap
    readonly property int move: reduce ? 0 : Tokens.move
    readonly property int swap: reduce ? 0 : Tokens.swap
    readonly property int flap: reduce ? 0 : Tokens.flap
    readonly property int ease: Tokens.ease
}
