import QtQuick
import Lithic

Item {
    id: toast

    property string text: ""
    property string glyph: ""
    // "info" | "success" | "error"
    property string kind: "info"
    // Auto-dismiss delay in ms; 0 keeps it until close().
    property int duration: 3200

    readonly property bool shown: toast._open
    signal dismissed()

    implicitWidth: box.implicitWidth
    implicitHeight: box.implicitHeight

    property bool _open: false

    function open(message) {
        if (message !== undefined)
            toast.text = message
        toast._open = true
        if (toast.duration > 0)
            life.restart()
        else
            life.stop()
    }
    function close() {
        toast._open = false
        life.stop()
        toast.dismissed()
    }

    Timer { id: life; interval: toast.duration; onTriggered: toast.close() }

    readonly property color _accent: toast.kind === "error" ? Theme.tertiary : Theme.surfaceText

    opacity: toast._open ? 1 : 0
    Behavior on opacity { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }

    transform: Translate {
        y: toast._open ? 0 : 8 * Theme.scale
        Behavior on y { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    }
    visible: opacity > 0.01

    CutRect {
        id: box
        anchors.fill: parent
        cut: LithicTokens.cut.placard * Theme.scale
        fill: Lithic.stone4
        strokeWidth: 1
        strokeColor: Lithic.vein
        implicitWidth: row.implicitWidth + 26 * Theme.scale
        implicitHeight: row.implicitHeight + 16 * Theme.scale

        Row {
            id: row
            anchors.centerIn: parent
            spacing: 9 * Theme.scale

            Text {
                visible: toast.glyph.length > 0
                anchors.verticalCenter: parent.verticalCenter
                text: toast.glyph
                font.family: Theme.icon
                font.pixelSize: Theme.fontField
                color: toast._accent
                renderType: Text.NativeRendering
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: toast.text
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontBody
                color: Theme.surfaceText
                renderType: Text.NativeRendering
            }
        }
    }
}
