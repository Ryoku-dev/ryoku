pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K

// The mode tabs at the right of the search line. The selected tab carries its
// own small bead that slides: a bone plate in Lithic, a tonal pill in Material,
// a rounded tint in Adwaita, and a short pill under the text in Fluent.
Item {
    id: root

    property var modes: []
    property string current: "all"

    property real beadX: 0
    property real beadW: 0
    readonly property bool animated: K.Look.microMotion && !K.Style.still

    signal picked(string mode)

    function label(mode) {
        switch (mode) {
        case "all": return I18n.tr("All");
        case "apps": return I18n.tr("Apps");
        case "windows": return I18n.tr("Windows");
        case "files": return I18n.tr("Files");
        case "actions": return I18n.tr("Actions");
        default: return mode;
        }
    }

    implicitWidth: tabs.implicitWidth
    implicitHeight: 30

    Item {
        id: bead
        readonly property bool underline: K.Look.fluent
        x: bead.underline ? root.beadX + (root.beadW - bead.width) / 2 : root.beadX
        y: bead.underline ? root.height - 3 : 0
        width: bead.underline ? 16 : root.beadW
        height: bead.underline ? 3 : root.height
        visible: root.beadW > 0

        Behavior on x {
            enabled: root.animated
            NumberAnimation {
                duration: K.Style.ms(K.Look.stateMs + 80)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Look.enterCurve
            }
        }
        Behavior on width {
            enabled: root.animated
            NumberAnimation {
                duration: K.Style.ms(K.Look.stateMs + 80)
                easing.type: Easing.BezierSpline
                easing.bezierCurve: K.Look.enterCurve
            }
        }

        K.Shape {
            anchors.fill: parent
            cutSize: K.Look.lithic ? K.Look.cutToken : K.Look.cutSm
            fill: K.Look.lithic ? K.Style.bone
                : K.Look.material ? K.Style.tint16
                : K.Look.fluent ? K.Style.ink : K.Style.tint10
        }
    }

    Row {
        id: tabs
        height: root.height
        spacing: K.Look.fluent ? K.Style.s2 : K.Style.s1

        Repeater {
            model: root.modes

            delegate: Item {
                id: tab

                required property var modelData
                required property int index
                readonly property bool selected: String(tab.modelData) === root.current

                function sync() {
                    if (tab.selected) {
                        root.beadX = tab.x;
                        root.beadW = tab.width;
                    }
                }

                implicitWidth: tabLabel.implicitWidth + (K.Look.fluent ? K.Style.s2 : K.Style.s3) * 2
                height: tabs.height

                onSelectedChanged: tab.sync()
                onXChanged: tab.sync()
                onWidthChanged: tab.sync()
                Component.onCompleted: tab.sync()

                K.Shape {
                    anchors.fill: parent
                    visible: !tab.selected && tabPointer.containsMouse && !K.Look.fluent
                    cutSize: K.Look.lithic ? K.Look.cutToken : K.Look.cutSm
                    fill: K.Style.tint5
                }

                Text {
                    id: tabLabel
                    anchors.centerIn: parent
                    text: root.label(String(tab.modelData))
                    color: tab.selected
                        ? (K.Look.lithic ? K.Style.inkOnBone : K.Style.ink)
                        : (tabPointer.containsMouse ? K.Style.inkDim : K.Style.inkMuted)
                    font.family: K.Look.lithic ? K.Style.mono : K.Style.font
                    font.pixelSize: K.Look.lithic ? K.Style.micro : K.Style.label
                    font.weight: tab.selected ? Font.DemiBold : Font.Medium
                    font.capitalization: K.Look.lithic ? Font.AllUppercase : Font.MixedCase
                    font.letterSpacing: K.Look.lithic ? K.Style.track : 0
                    renderType: Text.NativeRendering

                    Behavior on color {
                        enabled: !K.Style.still
                        ColorAnimation { duration: K.Style.snap }
                    }
                }

                MouseArea {
                    id: tabPointer
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.picked(String(tab.modelData))
                }
            }
        }
    }
}
