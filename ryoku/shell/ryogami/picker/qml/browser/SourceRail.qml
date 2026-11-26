import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// Where to look, as two groups
Flow {
    id: rail

    property var tabs: []
    property string current: ""
    signal selected(string id)
    signal setupRequested(string id)

    readonly property real s: Theme.scale
    spacing: 18 * rail.s

    function kindOf(id) {
        return (id === "steam" || id === "youtube" || id === "moewalls" || id === "motionbgs") ? "live" : "stills"
    }
    function glyphOf(id) {
        switch (id) {
        case "wallhaven": return "landscape"
        case "unsplash": return "photo_camera"
        case "pexels": return "photo_library"
        case "bing": return "today"
        case "repos": return "code"
        case "ryostore": return "storefront"
        case "steam": return "sports_esports"
        case "youtube": return "smart_display"
        case "moewalls": return "animation"
        case "motionbgs": return "motion_photos_on"
        }
        return "public"
    }
    function group(kind) {
        var out = []
        for (var i = 0; i < rail.tabs.length; ++i)
            if (rail.kindOf(rail.tabs[i].id) === kind) {
                var t = Object.assign({}, rail.tabs[i]); t.n = i; out.push(t)
            }
        return out
    }

    Repeater {
        model: [
            { kind: "stills", glyph: "image", label: I18n.tr("Stills") },
            { kind: "live", glyph: "play_circle", label: I18n.tr("Live walls") }
        ]
        Row {
            id: groupRow
            required property var modelData
            spacing: 4 * rail.s
            Row {
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6 * rail.s
                rightPadding: 6 * rail.s
                Glyph { name: groupRow.modelData.glyph; size: 16; ink: Lithic.fg3; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: groupRow.modelData.label
                    font.family: Theme.lithic ? Lithic.font.mono : Lithic.font.sans
                    font.pixelSize: Theme.fs(Theme.lithic ? 10 : 12)
                    font.letterSpacing: Theme.lithic ? 1.4 : 0
                    font.capitalization: Theme.lithic ? Font.AllUppercase : Font.MixedCase
                    color: Lithic.fg3
                    renderType: Text.NativeRendering
                }
            }
            Repeater {
                model: rail.group(groupRow.modelData.kind)
                CutRect {
                    id: chip
                    required property var modelData
                    readonly property bool on: rail.current === modelData.id
                    readonly property bool locked: modelData.enabled !== true
                    readonly property bool hovered: area.containsMouse
                    height: 38 * rail.s
                    width: face.implicitWidth + 26 * rail.s
                    cut: (LithicTokens.cut.small + (area.pressed ? Lithic.pressCutGrow : 0)) * rail.s
                    fill: chip.on ? Lithic.container : Lithic.alpha(Lithic.fg, chip.hovered ? 0.09 : 0.04)
                    Behavior on fill { ColorAnimation { duration: Theme.fast } }
                    Row {
                        id: face
                        anchors.centerIn: parent
                        anchors.verticalCenterOffset: area.pressed ? 1 : 0
                        spacing: 7 * rail.s
                        opacity: chip.locked && !chip.hovered ? 0.55 : 1
                        Glyph {
                            anchors.verticalCenter: parent.verticalCenter
                            name: rail.glyphOf(chip.modelData.id)
                            size: 18
                            ink: chip.on ? Lithic.containerInk : Lithic.fg2
                        }
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: chip.modelData.label
                            font.family: Lithic.font.sans
                            font.weight: Font.Medium
                            font.pixelSize: Theme.fs(13.5)
                            color: chip.on ? Lithic.containerInk : chip.hovered ? Lithic.fg : Lithic.fg2
                            renderType: Text.NativeRendering
                        }
                        Glyph {
                            visible: chip.locked
                            anchors.verticalCenter: parent.verticalCenter
                            name: chip.hovered ? "settings" : "lock"
                            size: 14
                            ink: Lithic.fg3
                        }
                    }
                    MouseArea {
                        id: area
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: chip.locked ? rail.setupRequested(chip.modelData.id) : rail.selected(chip.modelData.id)
                    }
                    CutRect {
                        visible: opacity > 0.01
                        opacity: chip.locked && chip.hovered && (chip.modelData.reason || "").length > 0 ? 1 : 0
                        Behavior on opacity { NumberAnimation { duration: Theme.fast } }
                        z: 30
                        anchors.top: parent.bottom
                        anchors.topMargin: 8 * rail.s
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: Math.min(300 * rail.s, why.implicitWidth + 20 * rail.s)
                        height: why.implicitHeight + 14 * rail.s
                        cut: LithicTokens.cut.tooltip * rail.s
                        fill: Lithic.fg
                        Text {
                            id: why
                            anchors.centerIn: parent
                            width: Math.min(280 * rail.s, implicitWidth)
                            wrapMode: Text.WordWrap
                            text: (chip.modelData.reason || "") + "  " + I18n.tr("Press to set it up.")
                            font.family: Lithic.font.sans
                            font.pixelSize: Theme.fs(12)
                            color: Lithic.ground
                            renderType: Text.NativeRendering
                        }
                    }
                }
            }
        }
    }
}
