import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// The right island
Row {
    id: island

    required property PickerState state
    required property var bar
    readonly property Item downloadButton: download
    readonly property Item settingsButton: settings

    spacing: 2 * Theme.scale

    IslandButton {
        id: palette
        anchors.verticalCenter: parent.verticalCenter
        bare: true
        width: 62 * Theme.scale
        implicitWidth: 62 * Theme.scale
        tooltip: island.bar.followWall ? I18n.tr("Colours follow the wallpaper") : I18n.tr("Take colours from the wallpaper")
        onTriggered: Settings.set("theme.policy", island.bar.followWall ? "off" : "wallpaper")
        CutRect {
            anchors.fill: parent
            cut: LithicTokens.cut.small * Theme.scale
            fill: island.bar.followWall ? Lithic.container
                : Lithic.alpha(Lithic.fg, palette.hovered ? 0.1 : 0.05)
            Behavior on fill { ColorAnimation { duration: Theme.fast } }
        }
        Repeater {
            model: [Lithic.sel, Lithic.gold, Lithic.fg]
            CutRect {
                required property color modelData
                required property int index
                x: (13 + index * (palette.hovered ? 13 : 10)) * Theme.scale
                y: 11 * Theme.scale
                width: 16 * Theme.scale
                height: 18 * Theme.scale
                cut: 3 * Theme.scale
                fill: modelData
                strokeWidth: 1.5 * Theme.scale
                strokeColor: island.bar.followWall ? Lithic.container : Lithic.stone2
                Behavior on x { NumberAnimation { duration: Theme.standard; easing.type: Easing.OutBack } }
            }
        }
    }

    IslandButton {
        anchors.verticalCenter: parent.verticalCenter
        visible: island.bar.show("filterBar.show.theme")
        glyph: island.bar.light ? "light_mode" : "dark_mode"
        tooltip: island.bar.light ? I18n.tr("Switch to dark") : I18n.tr("Switch to light")
        onTriggered: Settings.set("theme.mode", island.bar.light ? "dark" : "light")
    }

    Repeater {
        model: Tasks.bar
        delegate: TaskChip {
            required property var modelData
            anchors.verticalCenter: parent.verticalCenter
            task: modelData
        }
    }

    // Only videos and scenes carry sound, so these show only while one plays
    IslandButton {
        anchors.verticalCenter: parent.verticalCenter
        visible: island.bar.audioActive
        glyph: island.bar.muted ? "volume_off" : "volume_up"
        tooltip: island.bar.muted ? I18n.tr("Unmute wallpapers") : I18n.tr("Mute wallpapers")
        onTriggered: Settings.set("wallpaperMute", !island.bar.muted)
    }
    IslandButton {
        anchors.verticalCenter: parent.verticalCenter
        visible: island.bar.audioActive
        glyph: "tune"
        tooltip: I18n.tr("Audio mixer")
        on: island.state && island.state.sheet === "audio"
        onTriggered: if (island.state) island.state.openSheet("audio", undefined)
    }
    IslandButton {
        anchors.verticalCenter: parent.verticalCenter
        visible: island.bar.show("filterBar.show.playlists")
        glyph: "queue_music"
        soon: true
        tooltip: I18n.tr("Playlists are coming soon")
        on: island.state && island.state.sheet === "playlists"
        onTriggered: if (island.state) island.state.openSheet("playlists", undefined)
    }
    IslandButton {
        id: download
        anchors.verticalCenter: parent.verticalCenter
        visible: island.bar.show("filterBar.show.download")
        glyph: "travel_explore"
        label: I18n.tr("Discover")
        tooltip: I18n.tr("Find new wallpapers and live walls")
        on: island.state && island.state.sheet === "browser"
        onTriggered: if (island.state) {
            if (island.state.sheet === "browser") island.state.closeSheet()
            else island.state.openBrowser("")
        }
    }
    IslandButton {
        id: settings
        anchors.verticalCenter: parent.verticalCenter
        glyph: "settings"
        tooltip: I18n.tr("Settings")
        on: island.state && island.state.sheet === "settings"
        onTriggered: if (island.state) {
            if (island.state.sheet === "settings") island.state.closeSheet()
            else island.state.openSheet("settings", undefined)
        }
    }
}
