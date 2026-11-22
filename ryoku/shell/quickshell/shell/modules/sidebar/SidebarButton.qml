pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls as QQC
import Lithic
import "../ask/look"

QQC.AbstractButton {
    id: root

    property real s: 1
    property string glyph: ""
    property bool primary: false
    property bool armed: true
    property bool compact: false
    signal act()

    readonly property color rest: root.primary ? (QsLook.lithic ? Lithic.gold : Lithic.sel)
        : QsLook.lithic ? "transparent"
        : QsLook.material ? Lithic.field
        : QsLook.adwaita ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)
        : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.06 : 0.045)
    readonly property color ink: root.primary ? (QsLook.lithic ? Lithic.onGold : Lithic.onSel) : Lithic.fg
    readonly property real cutPx: (root.compact ? LithicTokens.cut.small : LithicTokens.cut.mid) * root.s

    enabled: root.armed
    hoverEnabled: true
    focusPolicy: Qt.StrongFocus
    implicitHeight: Math.round((root.compact ? 32 : 36) * root.s + (QsLook.material ? 4 * root.s : 0))
    implicitWidth: Math.ceil(Math.max(implicitHeight, content.implicitWidth + Math.round((root.text.length > 0 ? 28 : 12) * root.s)))
    opacity: root.enabled ? 1 : Lithic.disabledOpacity
    Accessible.name: root.text
    onClicked: root.act()

    background: CutRect {
        cut: root.cutPx + (QsLook.lithic && root.down ? Lithic.pressCutGrow * root.s : 0)
        fill: Qt.colorEqual(root.rest, "transparent")
            ? (root.down ? QsLook.press : root.hovered ? QsLook.hover : "transparent")
            : root.primary && QsLook.lithic ? Lithic.hoverFill(root.rest, root.down ? 0 : root.hovered ? 1 : 0)
            : Lithic.mix(root.rest, root.ink, root.down ? 0.12 : root.hovered ? 0.07 : 0)
        strokeColor: root.primary ? "transparent"
            : QsLook.lithic ? Lithic.vein
            : QsLook.fluent ? Lithic.alpha(Lithic.fg, QsLook.dark ? 0.09 : 0.12)
            : "transparent"
        strokeWidth: Math.max(1, Math.round(root.s))
        Behavior on fill {
            ColorAnimation { duration: QsLook.dur(root.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut) }
        }
        Behavior on cut {
            NumberAnimation { duration: QsLook.dur(LithicTokens.motion.press) }
        }
    }

    contentItem: Item {
        Row {
            id: content
            anchors.centerIn: parent
            anchors.verticalCenterOffset: QsLook.lithic && root.down ? Lithic.pressSink * root.s : 0
            spacing: Math.round(6 * root.s)

            Symbol {
                visible: root.glyph.length > 0
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                name: root.glyph
                size: 18
                ink: root.ink
            }
            Text {
                visible: root.text.length > 0
                anchors.verticalCenter: parent.verticalCenter
                text: root.text
                color: root.ink
                font.family: QsLook.sans
                font.pixelSize: Math.round(13 * root.s)
                font.weight: Font.Medium
                elide: Text.ElideRight
                renderType: Text.NativeRendering
            }
        }
    }

    FocusFrame {
        s: root.s
        shown: root.visualFocus
        cut: root.cutPx
    }
    CornerTip {
        s: root.s
        visible: root.hovered && root.text.length === 0 && root.Accessible.name.length > 0
        text: root.Accessible.name
    }
    HoverHandler {
        enabled: root.enabled
        cursorShape: Qt.PointingHandCursor
    }
}
