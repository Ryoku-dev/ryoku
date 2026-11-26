import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// A held or chosen control wears the selection wash, and confirm arms on the first press.
Item {
    id: btn

    property string label: ""
    property string glyph: ""
    // "toggle" holds the plate while active; "action" flashes it on press.
    property string mode: "toggle"
    property bool active: false
    property bool destructive: false
    property bool gold: false
    property bool soon: false
    property bool focused: false
    // Arm-then-confirm: first press arms, second within the window triggers.
    property bool confirm: false
    readonly property bool armed: btn.armState

    // A positive fixedWidth wins; otherwise the box hugs its content within [minWidth, maxWidth].
    property real fixedWidth: 0
    property real minWidth: 0
    property real maxWidth: 100000
    property real hpad: 14

    signal clicked()
    signal triggered()

    implicitHeight: 30 * Theme.scale
    implicitWidth: btn.fixedWidth > 0
        ? btn.fixedWidth
        : Math.max(btn.minWidth * Theme.scale,
                   Math.min(btn.maxWidth * Theme.scale, content.implicitWidth + btn.hpad * 2 * Theme.scale))

    readonly property color plate: btn.destructive ? Lithic.jasper : Lithic.container
    readonly property color plateInk: btn.destructive ? Lithic.ground : Lithic.containerInk
    readonly property bool hovered: hoverArea.containsMouse
    readonly property bool hot: btn.active || btn.holdFill || btn.armState

    property bool holdFill: false
    property bool armState: false

    Timer { id: flashTimer; interval: Theme.fast; onTriggered: btn.holdFill = false }
    Timer { id: disarmTimer; interval: 3000; onTriggered: btn.armState = false }
    function flash() { btn.holdFill = true; flashTimer.restart() }

    function _activate() {
        if (!btn.enabled || btn.soon)
            return
        if (btn.confirm) {
            if (btn.armState) {
                btn.armState = false
                disarmTimer.stop()
                btn.triggered()
                btn.clicked()
            } else {
                btn.armState = true
                disarmTimer.restart()
            }
            return
        }
        if (btn.mode === "action" && !btn.active)
            btn.flash()
        btn.triggered()
        btn.clicked()
    }

    readonly property bool _pressed: hoverArea.pressed && btn.enabled
    CutRect {
        id: box
        anchors.fill: parent
        readonly property real scale: 1
        cut: (LithicTokens.cut.small + (btn._pressed ? Lithic.pressCutGrow : 0)) * Theme.scale
        fill: !btn.enabled ? Theme.withAlpha(Lithic.fg, 0.04)
             : btn.gold ? (btn.hovered ? Qt.lighter(Lithic.gold, 1.08) : Lithic.gold)
             : btn.hot ? btn.plate
             : btn.hovered ? Theme.withAlpha(Lithic.fg, 0.11)
             : Theme.withAlpha(Lithic.fg, 0.06)
        strokeWidth: btn.focused ? 2 : (btn.destructive && !btn.hot ? 1 : 0)
        strokeColor: btn.focused ? Lithic.fg2 : Theme.withAlpha(Lithic.jasper, 0.55)
        Behavior on fill { ColorAnimation { duration: Theme.fast } }
        Behavior on cut { NumberAnimation { duration: 90 } }
    }

    Row {
        id: content
        anchors.centerIn: parent
        opacity: btn.soon ? 0.55 : 1
        spacing: 6 * Theme.scale
        anchors.verticalCenterOffset: btn._pressed ? 1 : 0

        readonly property color textColor: !btn.enabled
            ? Theme.withAlpha(Theme.surfaceText, 0.3)
            : btn.gold ? Lithic.onGold
            : btn.hot ? btn.plateInk
            : btn.destructive ? Lithic.jasper
            : btn.hovered ? Lithic.fg
            : Lithic.fg2

        Text {
            visible: btn.glyph.length > 0
            anchors.verticalCenter: parent.verticalCenter
            text: btn.glyph
            font.family: Theme.icon
            font.pixelSize: Theme.fontLabel
            color: content.textColor
            renderType: Text.NativeRendering
            Behavior on color { ColorAnimation { duration: Theme.fast } }
        }
        Text {
            visible: btn.soon
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Soon")
            font.family: Lithic.font.mono
            font.pixelSize: Theme.fs(9)
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1
            color: Lithic.fg3
            renderType: Text.NativeRendering
        }
        Text {
            visible: btn.label.length > 0 || btn.armState
            anchors.verticalCenter: parent.verticalCenter
            text: btn.armState ? (I18n.tr("Confirm") + " " + btn.label) : btn.label
            font.family: Theme.sans
            font.weight: btn.hot ? Font.DemiBold : Font.Medium
            font.pixelSize: Theme.fontLabel
            color: content.textColor
            renderType: Text.NativeRendering
            Behavior on color { ColorAnimation { duration: Theme.fast } }
        }
    }

    MouseArea {
        id: hoverArea
        anchors.fill: parent
        hoverEnabled: true
        enabled: btn.enabled
        cursorShape: btn.soon ? Qt.ArrowCursor : Qt.PointingHandCursor
        onClicked: btn._activate()
    }
}
