import QtQuick
import QtQuick.Effects
import Lithic
import Ryoku.Ui.Singletons

// The left island
Row {
    id: island

    required property PickerState state
    readonly property string current: island.state ? island.state.collection : ""

    readonly property var libraries: [
        { value: "wallpapers", glyph: "wallpaper", label: I18n.tr("Wallpapers") },
        { value: "themes", glyph: "palette", label: I18n.tr("Themes") },
        { value: "rices", glyph: "format_paint", label: I18n.tr("Rices") },
        { value: "workshop", glyph: "view_in_ar", label: I18n.tr("Workshop") }
    ]

    spacing: 2 * Theme.scale

    Item {
        width: 44 * Theme.scale
        height: 40 * Theme.scale
        anchors.verticalCenter: parent.verticalCenter
        Image {
            id: mark
            anchors.centerIn: parent
            source: Qt.resolvedUrl("brand/logo-mark.png")
            height: 24 * Theme.scale
            width: height * 1.15
            fillMode: Image.PreserveAspectFit
            sourceSize.height: 64
            smooth: true
            mipmap: true
            visible: false
        }
        // The mark takes the ink, so it reads on light paper as well as dark
        MultiEffect {
            anchors.fill: mark
            source: mark
            colorization: 1
            colorizationColor: Lithic.fg
        }
    }

    Repeater {
        model: island.libraries
        IslandButton {
            required property var modelData
            anchors.verticalCenter: parent.verticalCenter
            glyph: modelData.glyph
            label: modelData.label
            labelWhenOn: true
            on: island.current === modelData.value
            tooltip: on ? "" : modelData.label
            onTriggered: if (island.state) island.state.setCollection(modelData.value)
        }
    }
}
