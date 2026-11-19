import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// The panel's head. Lithic: a "◁ MARKUP" quiet plaque, then BEAUTIFY in mono
// caps with the brand seal stamped beside it. Material: a back icon button and
// the headline. Adwaita: a header bar, back at the start, the bold title
// centred, the suggested Copy at the end. Fluent: a CommandBar, back and the
// subtitle, then Save and the accent Copy.
Item {
    id: hd

    required property var studio
    property real u: 1

    readonly property bool busy: hd.studio ? hd.studio.busy : false
    readonly property string copyText: hd.busy ? I18n.tr("Working\u2026") : I18n.tr("Copy")

    implicitHeight: Math.round((Look.lithic ? 52 : Look.material ? 64 : Look.adwaita ? 47 : 52) * hd.u)
    height: implicitHeight

    // ---- Lithic
    TextButton {
        visible: Look.lithic
        x: Math.round(16 * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        u: hd.u
        cellHeight: 24
        text: "\u25c1 " + I18n.tr("Markup")
        keyHint: "ESC"
        onClicked: hd.studio.back()
    }
    Row {
        visible: Look.lithic
        anchors.right: parent.right
        anchors.rightMargin: Math.round(16 * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(10 * hd.u)
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Beautify").toUpperCase()
            color: Look.fg
            font.family: Look.mono
            font.pixelSize: Math.round(11 * hd.u)
            font.weight: Font.Bold
            font.letterSpacing: Look.tracking * 1.5 * hd.u
            renderType: Text.CurveRendering
        }
        // the seal, stamped: the mark in the paper's ink on a cut sel plate, set
        // a hair off square as a hand stamp lands
        CutRect {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.round(22 * hd.u)
            height: width
            rotation: -4
            design: "lithic"
            cut: 3 * hd.u
            fill: Look.sel
            antialiasing: true
            Text {
                anchors.centerIn: parent
                text: hd.studio && hd.studio.mark ? hd.studio.mark : "\u529b"
                color: Look.onSel
                font.family: Look.jp
                font.pixelSize: Math.round(14 * hd.u)
                font.weight: Font.Bold
                renderType: Text.CurveRendering
            }
        }
    }
    Rectangle {
        visible: Look.lithic
        x: Math.round(16 * hd.u)
        anchors.bottom: parent.bottom
        width: parent.width - 2 * x
        height: Math.max(1, Math.round(hd.u))
        color: Look.vein
    }

    // ---- Material
    GlyphButton {
        id: m3Back
        visible: Look.material
        x: Math.round(12 * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: Math.round(4 * hd.u)
        u: hd.u
        cellWidth: 40
        cellHeight: 40
        iconSize: 22
        icon: "back"
        ink: Look.fg2
        tip: I18n.tr("Back")
        keyHint: "Esc"
        tipBelow: true
        onClicked: hd.studio.back()
    }
    Text {
        visible: Look.material
        anchors.left: m3Back.right
        anchors.leftMargin: Math.round(8 * hd.u)
        anchors.verticalCenter: m3Back.verticalCenter
        text: I18n.tr("Beautify")
        color: Look.fg
        font.family: Look.sans
        font.pixelSize: Math.round(22 * hd.u)
        renderType: Text.CurveRendering
    }

    // ---- Adwaita and Fluent
    GlyphButton {
        id: back
        visible: Look.adwaita || Look.fluent
        x: Math.round((Look.adwaita ? 6 : 8) * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        u: hd.u
        cellWidth: Look.adwaita ? 34 : 36
        cellHeight: Look.adwaita ? 34 : 36
        iconSize: Look.adwaita ? 16 : 16
        icon: "back"
        ink: Look.fg
        tip: I18n.tr("Back")
        keyHint: "Esc"
        tipBelow: true
        onClicked: hd.studio.back()
    }
    Text {
        visible: Look.adwaita
        anchors.centerIn: parent
        text: I18n.tr("Beautify")
        color: Look.fg
        font.family: Look.sans
        font.pixelSize: Math.round(14 * hd.u)
        font.weight: Font.Bold
        renderType: Text.CurveRendering
    }
    Text {
        visible: Look.fluent
        anchors.left: back.right
        anchors.leftMargin: Math.round(8 * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        text: I18n.tr("Beautify")
        color: Look.fg
        font.family: Look.sans
        font.pixelSize: Math.round(20 * hd.u)
        font.weight: Font.DemiBold
        renderType: Text.CurveRendering
    }
    Row {
        visible: Look.adwaita || Look.fluent
        anchors.right: parent.right
        anchors.rightMargin: Math.round((Look.adwaita ? 6 : 10) * hd.u)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(8 * hd.u)
        TextButton {
            visible: Look.fluent
            u: hd.u
            text: I18n.tr("Save")
            keyHint: "Ctrl+S"
            tipBelow: true
            enabled: !hd.busy
            onClicked: hd.studio.save()
        }
        TextButton {
            u: hd.u
            primary: true
            text: hd.copyText
            keyHint: "Ctrl+C"
            tipBelow: true
            enabled: !hd.busy
            onClicked: hd.studio.copy()
        }
    }
    // Adwaita's header bar keeps a shade line under it once the body scrolls
    property bool scrolled: false
    Rectangle {
        visible: Look.adwaita
        anchors.bottom: parent.bottom
        width: parent.width
        height: Math.max(1, Math.round(hd.u))
        color: Lithic.alpha(Look.fg, Look.dark ? 0.1 : 0.12)
        opacity: hd.scrolled ? 1 : 0
        Behavior on opacity {
            NumberAnimation { duration: Look.dur(120) }
        }
    }
}
