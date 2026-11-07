pragma ComponentBehavior: Bound

// One dropdown shared by the agent and period pickers: options, a current
// marker, hover and a picked signal. The page owns the veil and the position,
// so the menu only draws itself.
import QtQuick
import Quickshell
import Quickshell.Widgets
import inir.modules.common
import inir.modules.iris.style
import inir.modules.iris.components

IrisSurface {
    id: root

    property bool open: false
    property var options: []
    property string currentValue: ""
    property real menuWidth: 172
    property int rowHeight: Math.round(30 * IrisStyle.density)
    property bool boldCurrent: true
    signal picked(string value)

    visible: root.open
    raised: true
    outlined: true
    width: Math.round(root.menuWidth * IrisStyle.density)
    height: column.implicitHeight + 2 * Math.round(6 * IrisStyle.density)
    radius: IrisStyle.radiusTile

    Rectangle {
        anchors.fill: parent
        radius: root.radius
        color: "transparent"
        border.width: 1
        border.color: IrisStyle.borderStrong
    }

    Column {
        id: column
        x: Math.round(6 * IrisStyle.density)
        y: Math.round(6 * IrisStyle.density)
        width: root.width - 2 * Math.round(6 * IrisStyle.density)

        Repeater {
            model: root.options
            Item {
                id: row
                required property var modelData
                width: column.width
                height: root.rowHeight
                readonly property bool current: String(modelData?.value ?? "") === root.currentValue

                Rectangle {
                    anchors.fill: parent
                    radius: IrisStyle.radiusSmall
                    color: row.current || rowArea.containsMouse
                        ? IrisStyle.fillHover : "transparent"
                }
                IrisText {
                    anchors.left: parent.left
                    anchors.leftMargin: Math.round(9 * IrisStyle.density)
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(row.modelData?.label ?? "")
                    font.pixelSize: IrisStyle.typeLabel
                    font.weight: IrisStyle.weight(
                        row.current && root.boldCurrent ? Font.DemiBold : Font.Medium)
                    color: row.current ? IrisStyle.accent : IrisStyle.text
                }
                MaterialSymbol {
                    anchors.right: parent.right
                    anchors.rightMargin: Math.round(9 * IrisStyle.density)
                    anchors.verticalCenter: parent.verticalCenter
                    visible: row.current
                    text: "check"
                    iconSize: Math.round(14 * IrisStyle.density)
                    color: IrisStyle.accent
                }
                MouseArea {
                    id: rowArea
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        root.open = false
                        root.picked(String(row.modelData?.value ?? ""))
                    }
                }
            }
        }
    }
}
