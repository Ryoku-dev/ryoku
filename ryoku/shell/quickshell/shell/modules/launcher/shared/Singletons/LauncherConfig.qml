pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io

Singleton {
    id: root

    property alias variant:      adapter.variant
    property alias radius:       adapter.radius
    property alias bgBlur:      adapter.bgBlur
    property alias heroImage:    adapter.heroImage
    property alias heroStrength: adapter.heroStrength
    property alias heroPosX:     adapter.heroPosX
    property alias heroPosY:     adapter.heroPosY
    property alias showWeather:  adapter.showWeather
    property alias showGreeting: adapter.showGreeting
    property alias resultSettleMs: adapter.resultSettleMs
    property alias horizonMode:  adapter.horizonMode
    property alias horizonColor: adapter.horizonColor
    property alias overlayOpacity: adapter.overlayOpacity
    property alias hiddenApps: adapter.hiddenApps
    property alias columns: adapter.columns
    property alias matWidth: adapter.matWidth
    property alias sectionOrder: adapter.sectionOrder
    property alias hiddenSections: adapter.hiddenSections
    property alias pinnedApps: adapter.pinnedApps

    FileView {
        id: file
        path: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku/launcher.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        atomicWrites: true
        onFileChanged: reload()

        JsonAdapter {
            id: adapter
            property string variant: "hero"
            property real radius: 16
            property int bgBlur: 2
            property string heroImage: ""
            property real heroStrength: 0.6
            property real heroPosX: 0.5
            property real heroPosY: 0.5
            property bool showWeather: true
            property bool showGreeting: true
            property int resultSettleMs: 360
            property string horizonMode: "auto"
            property string horizonColor: "#ffc777"
            property real overlayOpacity: 0.94
            property var hiddenApps: []
            property int columns: 8
            property int matWidth: 144
            property var sectionOrder: []
            property var hiddenSections: []
            property var pinnedApps: []
        }
    }

    Component.onCompleted: if (!file.text()) file.writeAdapter();
}
