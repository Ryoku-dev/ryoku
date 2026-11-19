import QtQuick
import QtQuick.Controls as Controls
import Ryoku.Ui.Singletons
import Lithic
import "Singletons"

CutRect {
    id: button
    property string iconName: ""
    property string label: ""
    property bool selected: false
    signal clicked()
    signal dragRequested()

    width: 32
    height: 32
    design: Tokens.design
    cut: Tokens.radius
    fill: selected ? Tokens.bone : mouse.pressed ? Tokens.tint16 : mouse.containsMouse ? Tokens.tint10 : "transparent"
    strokeColor: selected ? Tokens.bone : Theme.hair
    strokeWidth: 1
    scale: mouse.pressed ? 0.94 : 1
    Behavior on fill { ColorAnimation { duration: Theme.snap } }
    Behavior on scale { NumberAnimation { duration: Theme.snap } }
    Accessible.role: Accessible.Button
    Accessible.name: label
    Accessible.onPressAction: clicked()
    Controls.ToolTip.visible: mouse.containsMouse && !mouse.pressed
    Controls.ToolTip.text: label
    Controls.ToolTip.delay: 500

    Glyph {
        anchors.centerIn: parent
        name: button.iconName
        size: 16
        ink: button.selected ? Tokens.inkOnBone : Theme.ink
    }
    MouseArea {
        id: mouse
        anchors.fill: parent
        hoverEnabled: true
        preventStealing: true
        cursorShape: Qt.PointingHandCursor
        property point pressPoint
        property bool dragged: false
        onPressed: event => { pressPoint = Qt.point(event.x, event.y); dragged = false; }
        onPositionChanged: event => {
            if (!pressed || dragged || Math.abs(event.x - pressPoint.x) + Math.abs(event.y - pressPoint.y) < 8) return;
            dragged = true;
            button.dragRequested();
        }
        onClicked: if (!dragged) button.clicked()
    }
}
