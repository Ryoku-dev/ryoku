import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: root

    required property PickerState state

    anchors.fill: parent

    readonly property bool shown: root.state ? root.state.themeBarOpen : false

    property real reveal: 0
    states: State { name: "open"; when: root.shown; PropertyChanges { target: root; reveal: 1 } }
    Behavior on reveal { NumberAnimation { duration: Theme.standard; easing.type: Theme.revealEasing } }
    visible: reveal > 0.01

    Keys.onEscapePressed: if (root.state) root.state.themeBarOpen = false

    // The daemon applies the theme from settings and the masthead's wallpaper-colours toggle
    // reads the same policy, so the lit tile is derived from settings rather than tracked here.
    SettingValue { id: policySetting; key: "theme.policy" }
    SettingValue { id: staticSetting; key: "theme.staticTheme" }
    readonly property string activeId: String(policySetting.value) === "wallpaper"
        ? "Wallpaper" : String(staticSetting.value || "")

    function applyTheme(id) {
        if (id === "Wallpaper") {
            Settings.set("theme.policy", "wallpaper")
        } else {
            Settings.set("theme.staticTheme", id)
            Settings.set("theme.policy", "fixed")
        }
        if (root.state) root.state.toast(I18n.tr("Applying theme"), "info")
    }
    function followWallpaper() { root.applyTheme("Wallpaper") }

    LibraryView { id: themes; collection: "themes" }

    Rectangle {
        id: panel
        width: parent.width - 44 * Theme.scale
        height: 168 * Theme.scale
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: 88 * Theme.scale
        radius: Theme.radius
        opacity: root.reveal
        transform: Translate { y: (1 - root.reveal) * 12 * Theme.scale }

        color: Theme.withAlpha(Theme.surface, 0.97)
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.55)

        MouseArea { anchors.fill: parent }

        Column {
            anchors.fill: parent
            anchors.topMargin: 12 * Theme.scale
            anchors.bottomMargin: 12 * Theme.scale
            anchors.leftMargin: 18 * Theme.scale
            anchors.rightMargin: 18 * Theme.scale
            spacing: 12 * Theme.scale

            Item {
                width: parent.width
                height: 30 * Theme.scale

                Column {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 1
                    Text {
                        text: I18n.tr("Theme")
                        font.family: Theme.display
                        font.pixelSize: Theme.fontHead
                        color: Theme.surfaceText
                        renderType: Text.NativeRendering
                    }
                    Text {
                        text: I18n.tr("Matugen")
                        font.family: Theme.sans
                        font.weight: Font.Medium
                        font.pixelSize: Theme.fontFine
                        color: Theme.withAlpha(Theme.surfaceText, 0.55)
                        renderType: Text.NativeRendering
                    }
                }

                Row {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: 7 * Theme.scale

                    FixedButton {
                        mode: "action"
                        label: I18n.tr("Follow wallpaper")
                        active: root.activeId === "Wallpaper"
                        onTriggered: root.followWallpaper()
                    }
                    FixedButton {
                        mode: "action"
                        label: I18n.tr("Designer")
                        soon: true
                        onTriggered: if (root.state) root.state.openSheet("themeDesigner", undefined)
                    }
                    FixedButton {
                        mode: "action"
                        label: I18n.tr("Audition")
                        onTriggered: if (root.state) root.state.openSheet("themeAudition", undefined)
                    }
                    FixedButton {
                        mode: "action"
                        glyph: "\u00d7"
                        minWidth: 30
                        onTriggered: if (root.state) root.state.themeBarOpen = false
                    }
                }
            }

            ListView {
                id: strip
                width: parent.width
                height: parent.height - 30 * Theme.scale - 12 * Theme.scale
                orientation: ListView.Horizontal
                spacing: 10 * Theme.scale
                clip: true
                boundsBehavior: Flickable.StopAtBounds
                model: themes

                delegate: Item {
                    id: tile
                    required property string key
                    required property string name
                    readonly property var entry: Library.entry("themes", key)
                    readonly property bool active: root.activeId === key
                    readonly property bool hovered: tileMouse.containsMouse

                    width: 130 * Theme.scale
                    height: strip.height

                    Rectangle {
                        anchors.fill: parent
                        radius: Theme.radius
                        color: tile.active ? Theme.withAlpha(Theme.surfaceText, 0.09)
                             : tile.hovered ? Theme.withAlpha(Theme.surfaceText, 0.05)
                             : "transparent"
                        border.width: tile.active ? 2 : 1
                        border.color: tile.active ? Theme.surfaceText
                                     : tile.hovered ? Theme.withAlpha(Theme.surfaceText, 0.28)
                                     : Theme.withAlpha(Theme.outline, 0.4)
                        Behavior on color { ColorAnimation { duration: Theme.fast } }

                        Column {
                            anchors.fill: parent
                            anchors.margins: 8 * Theme.scale
                            spacing: 7 * Theme.scale

                            Item {
                                width: parent.width
                                height: parent.height - labelText.implicitHeight - 7 * Theme.scale
                                clip: true

                                Image {
                                    anchors.fill: parent
                                    visible: tile.entry && tile.entry.preview && status === Image.Ready
                                    source: Library.fileUrl(tile.entry && tile.entry.preview ? tile.entry.preview : "")
                                    fillMode: Image.PreserveAspectCrop
                                    asynchronous: true
                                    cache: false
                                    sourceSize.width: 260
                                    sourceSize.height: 180
                                }
                                Row {
                                    anchors.fill: parent
                                    visible: !(tile.entry && tile.entry.preview)
                                    spacing: 0
                                    Repeater {
                                        model: (tile.entry && tile.entry.swatches) ? tile.entry.swatches.slice(0, 6) : []
                                        delegate: Rectangle {
                                            required property var modelData
                                            width: parent.width / Math.max(1, Math.min(6, (tile.entry && tile.entry.swatches) ? tile.entry.swatches.length : 1))
                                            height: parent.height
                                            color: modelData
                                        }
                                    }
                                }
                            }

                            Text {
                                id: labelText
                                width: parent.width
                                text: tile.name
                                font.family: Theme.sans
                                font.weight: tile.active ? Font.DemiBold : Font.Medium
                                font.pixelSize: Theme.fontBase
                                color: Theme.surfaceText
                                elide: Text.ElideRight
                                horizontalAlignment: Text.AlignHCenter
                                renderType: Text.NativeRendering
                            }
                        }

                        MouseArea {
                            id: tileMouse
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.applyTheme(tile.key)
                        }
                    }
                }
            }

            Text {
                visible: themes.count === 0
                width: parent.width
                text: I18n.tr("No themes installed")
                font.family: Theme.sans
                font.weight: Font.Medium
                font.pixelSize: Theme.fontLabel
                color: Theme.withAlpha(Theme.surfaceText, 0.6)
                horizontalAlignment: Text.AlignHCenter
                renderType: Text.NativeRendering
            }
        }
    }
}
