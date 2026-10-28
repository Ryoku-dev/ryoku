import QtQuick
import Ryoku.Ui.Singletons

// The four libraries as masthead tabs: the Latin name with its Japanese word above it.
// One bone plate sits under the open tab and slides to the next. The plate carries its
// own dark copy of the words, clipped to itself, so every letter it passes over inverts
// in step with it. Switching is instant: it only reconfigures the one shared view.
Item {
    id: tabs

    required property PickerState state

    readonly property var _tabs: [
        { value: "wallpapers", label: I18n.tr("Wallpapers"), jp: "壁紙" },
        { value: "themes", label: I18n.tr("Themes"), jp: "配色" },
        { value: "rices", label: I18n.tr("Rices"), jp: "装い" },
        { value: "workshop", label: I18n.tr("Workshop"), jp: "工房" }
    ]
    readonly property string current: tabs.state ? tabs.state.collection : ""
    property string hoveredValue: ""

    property real plateX: 0
    property real plateW: 0
    property bool _placed: false

    implicitWidth: base.implicitWidth
    implicitHeight: 42 * Theme.scale

    Component {
        id: face
        Item {
            id: tab
            required property var modelData
            readonly property bool inverted: parent !== null && parent.inverted === true
            readonly property bool active: tabs.current === modelData.value
            readonly property bool hovered: tabs.hoveredValue === modelData.value && !tab.active

            // Sized for the bold cut, so a tab keeps its width as it becomes the open one.
            width: Math.max(boldCut.advanceWidth, kanji.implicitWidth) + 30 * Theme.scale
            height: tabs.height

            TextMetrics {
                id: boldCut
                font.family: Theme.sans
                font.weight: Font.DemiBold
                font.pixelSize: Theme.fs(14)
                text: tab.modelData.label
            }
            Binding {
                when: tab.active && !tab.inverted
                restoreMode: Binding.RestoreNone
                target: tabs; property: "plateX"; value: tab.x
            }
            Binding {
                when: tab.active && !tab.inverted
                restoreMode: Binding.RestoreNone
                target: tabs; property: "plateW"; value: tab.width
            }

            Text {
                id: kanji
                anchors.horizontalCenter: parent.horizontalCenter
                y: (tab.hovered ? 4 : 6) * Theme.scale
                text: UiPreferences.text(tab.modelData.jp, "")
                font.family: Theme.jp
                font.pixelSize: Theme.fs(9.5)
                font.letterSpacing: 2
                color: tab.inverted ? Theme.withAlpha(Theme.surface, 0.7)
                     : Theme.withAlpha(Theme.surfaceText, tab.hovered ? 0.85 : 0.42)
                renderType: Text.NativeRendering
                Behavior on y { NumberAnimation { duration: Theme.fast; easing.type: Easing.OutCubic } }
                Behavior on color { ColorAnimation { duration: Theme.fast } }
            }
            Text {
                id: latin
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                anchors.bottomMargin: 6 * Theme.scale
                text: tab.modelData.label
                font.family: Theme.sans
                font.weight: tab.active ? Font.DemiBold : Font.Medium
                font.pixelSize: Theme.fs(14)
                color: tab.inverted ? Theme.surface
                     : Theme.withAlpha(Theme.surfaceText, tab.active || tab.hovered ? 1 : 0.66)
                renderType: Text.NativeRendering
                Behavior on color { ColorAnimation { duration: Theme.fast } }
            }
            // A hairline draws under a tab the pointer rests on.
            Rectangle {
                visible: !tab.inverted
                anchors.horizontalCenter: latin.horizontalCenter
                anchors.top: latin.bottom
                anchors.topMargin: 1
                height: 1
                width: tab.hovered ? latin.width : 0
                color: Theme.withAlpha(Theme.surfaceText, 0.6)
                Behavior on width { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutCubic } }
            }

            MouseArea {
                visible: !tab.inverted
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onContainsMouseChanged: {
                    if (containsMouse) tabs.hoveredValue = tab.modelData.value
                    else if (tabs.hoveredValue === tab.modelData.value) tabs.hoveredValue = ""
                }
                onClicked: if (tabs.state) tabs.state.setCollection(tab.modelData.value)
            }
        }
    }

    Row {
        id: base
        readonly property bool inverted: false
        height: parent.height
        Repeater { model: tabs._tabs; delegate: face }
    }

    Rectangle {
        id: plate
        x: tabs.plateX
        width: tabs.plateW
        height: parent.height
        radius: Theme.radius
        color: Theme.surfaceText
        clip: true
        visible: tabs.plateW > 0
        Behavior on x {
            enabled: tabs._placed
            NumberAnimation { duration: Theme.standard * 1.4; easing.type: Easing.OutBack; easing.overshoot: 0.8 }
        }
        Behavior on width {
            enabled: tabs._placed
            NumberAnimation { duration: Theme.standard * 1.4; easing.type: Easing.OutCubic }
        }

        Row {
            readonly property bool inverted: true
            x: -plate.x
            height: tabs.height
            Repeater { model: tabs._tabs; delegate: face }
        }
    }
    // The first placement lands without travelling; only a switch slides.
    Timer { interval: 1; running: tabs.plateW > 0 && !tabs._placed; onTriggered: tabs._placed = true }
}
