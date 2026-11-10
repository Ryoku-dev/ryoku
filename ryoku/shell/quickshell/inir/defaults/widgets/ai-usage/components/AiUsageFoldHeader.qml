pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Widgets
import inir
import inir.services
import inir.modules.common
import inir.modules.common.widgets
import inir.modules.common.functions
import inir.modules.iris.style
import inir.modules.iris.components

Item {
    id: header
    property string title: ""
    property bool expanded: false
    signal toggled()
    default property alias accessories: accessoryRow.data
    width: parent ? parent.width : 0
    height: Math.max(Math.round(24 * IrisStyle.density), titleText.implicitHeight,
        accessoryRow.implicitHeight, chevronIcon.implicitHeight)

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: header.toggled()
    }
    IrisText {
        id: titleText
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: header.title
        font.pixelSize: Math.round(12 * IrisStyle.density)
        font.weight: IrisStyle.weight(Font.DemiBold)
    }
    Row {
        id: accessoryRow
        anchors.right: chevronIcon.left
        anchors.rightMargin: Math.round(8 * IrisStyle.density)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(6 * IrisStyle.density)
    }
    MaterialSymbol {
        id: chevronIcon
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: "expand_more"
        iconSize: Math.round(16 * IrisStyle.density)
        color: IrisStyle.subtext
        rotation: header.expanded ? 180 : 0
        Behavior on rotation {
            NumberAnimation {
                duration: IrisStyle.duration(160)
                easing.type: Easing.BezierSpline
            }
        }
    }
}
