import QtQuick
import QtQuick.Templates as T
import Lithic
import "../../Singletons"

// the open page's sections, hung under its line like a file's symbols: the one being read is
// marked as the page scrolls, and choosing one brings it to the top of the page
Item {
    id: root

    required property var hub
    required property var tree
    property bool shown: false
    property bool animate: true
    readonly property var sections: root.hub.outline
    readonly property int current: root.hub.outlineIndex
    readonly property real rowHeight: 26

    width: parent ? parent.width : 0
    height: root.shown && root.sections.length >= 2 ? list.implicitHeight + 4 : 0
    clip: true
    visible: height > 0
    Behavior on height {
        enabled: root.animate
        NumberAnimation { duration: Tone.unfold; easing.type: Easing.OutCubic }
    }

    // the thread the sections hang on, from under the page's mark to the last of them; every
    // part is placed by anchors, so a right-to-left language mirrors it
    Rectangle {
        anchors.left: parent.left
        anchors.leftMargin: 36
        y: 0
        width: 1
        height: Math.max(0, list.implicitHeight - root.rowHeight / 2)
        color: Tone.guide
    }

    Column {
        id: list
        width: parent.width

        Repeater {
            model: root.sections
            delegate: T.AbstractButton {
                id: line
                required property string modelData
                required property int index
                readonly property bool here: line.index === root.current
                width: list.width
                height: root.rowHeight
                hoverEnabled: true
                focusPolicy: Qt.TabFocus
                Accessible.role: Accessible.Button
                Accessible.name: line.modelData
                onClicked: root.hub.chooseSection(line.index)
                Keys.onReturnPressed: line.clicked()
                Keys.onEnterPressed: line.clicked()
                Keys.onUpPressed: root.tree.step(line, -1)
                Keys.onDownPressed: root.tree.step(line, 1)

                background: Item {
                    CutRect {
                        anchors.fill: parent
                        anchors.leftMargin: 46
                        anchors.rightMargin: 4
                        cut: Tone.rowCut
                        fill: line.down ? Tone.press : Tone.hover
                        opacity: line.hovered || line.visualFocus ? 1 : 0
                        strokeWidth: line.visualFocus ? 1 : 0
                        strokeColor: Lithic.fg2
                        Behavior on opacity { NumberAnimation { duration: Tone.quick } }
                    }
                }

                contentItem: Item {
                    Rectangle {
                        id: node
                        anchors.left: parent.left
                        anchors.leftMargin: 36 - width / 2 + 0.5
                        anchors.verticalCenter: parent.verticalCenter
                        width: line.here ? 7 : 5
                        height: width
                        radius: width / 2
                        color: line.here ? Tone.accent : Tone.side
                        border.width: line.here ? 0 : 1
                        border.color: line.hovered ? Lithic.fg2 : Tone.guide
                        Behavior on width { NumberAnimation { duration: Tone.move; easing.type: Easing.OutBack; easing.overshoot: 2 } }
                        Behavior on color { ColorAnimation { duration: Tone.quick } }
                    }
                    Label {
                        anchors.left: parent.left
                        anchors.leftMargin: 55 + (line.hovered && !line.here ? 2 : 0)
                        anchors.right: parent.right
                        anchors.rightMargin: 10
                        anchors.verticalCenter: parent.verticalCenter
                        text: line.modelData
                        typeRole: "caption"
                        ink: line.here ? Lithic.fg : line.hovered ? Lithic.fg2 : Lithic.fg3
                        font.weight: line.here ? Font.DemiBold : Font.Normal
                        Behavior on anchors.leftMargin { NumberAnimation { duration: Tone.move; easing.type: Easing.OutCubic } }
                        Behavior on ink { ColorAnimation { duration: Tone.quick } }
                    }
                }
            }
        }
    }
}
