pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Singleton {
    readonly property color panelSolid: Tokens.paperLift
    readonly property color hair: Tokens.lineSoft
    readonly property color ink: Tokens.ink
    readonly property string ui: Tokens.ui
    readonly property string mono: Tokens.mono

    readonly property int snap: Tokens.snap
}
