import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../about"

// the sheet's colophon at the page's foot, in the smallest mono: whether every change is on
// the desktop yet, where the pointer stands on the sheet, and the page's code in bars
Item {
    id: root

    required property var hub
    // the pointer on the sheet, or null while it is elsewhere
    property var pointer: null

    readonly property bool settled: !Changes.unsettled
    readonly property string coord: {
        var four = n => ("0000" + Math.max(0, Math.round(n))).slice(-4);
        return root.pointer ? "X " + four(root.pointer.x) + "   Y " + four(root.pointer.y) : "X ----   Y ----";
    }

    implicitHeight: 26

    Row {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        spacing: 8

        // a square of state: hollow while a change is on its way, filled once it is live
        Rectangle {
            anchors.verticalCenter: parent.verticalCenter
            width: 7
            height: 7
            color: root.settled ? Lithic.jade : "transparent"
            border.width: root.settled ? 0 : 1
            border.color: Lithic.amber
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.settled ? I18n.tr("Saved, live on your desktop") : I18n.tr("Saving")
            typeRole: "monoSm"
            inkRole: "fg2"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.2
        }
    }

    Label {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.verticalCenter: parent.verticalCenter
        text: root.coord
        typeRole: "monoSm"
        inkRole: root.pointer ? "fg2" : "fg3"
        font.letterSpacing: 1.2
    }

    CardBarcode {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.hub.pageId || "home"
        ink: Lithic.fg3
        barHeight: 9
        unit: 1
    }
}
