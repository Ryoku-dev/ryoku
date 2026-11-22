pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell.Widgets
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

Item {
    id: root

    required property real s
    required property bool active

    readonly property var player: active ? Media.player : null
    // follow the player itself, not the gated one, so the card doesn't fold and unfold on every open
    readonly property bool playing: Media.player !== null
    readonly property bool showArtwork: SidebarState.elementVisible("mediaArtwork")
    readonly property bool showTrack: SidebarState.elementVisible("mediaTrack")
    readonly property bool showTransport: SidebarState.elementVisible("mediaTransport")
    readonly property string coverSource: root.player && root.showArtwork ? Music.coverFor(root.player) : ""
    readonly property string title: root.player && root.player.trackTitle ? root.player.trackTitle : ""
    readonly property string artists: root.player
        ? Theme.joinArtists(root.player.trackArtists, root.player.trackArtist) : ""
    readonly property real pad: Math.round(10 * root.s)
    readonly property real artSide: Math.round(52 * root.s)
    readonly property real artCut: LithicTokens.cut.small * root.s
    readonly property bool artReady: (QsLook.lithic ? cutCover.status : roundCover.status) === Image.Ready

    property real shownHeight: Math.round((root.playing ? 72 : 40) * root.s)
    Behavior on shownHeight {
        NumberAnimation { duration: QsLook.dur(QsLook.unfoldMs); easing.type: Easing.BezierSpline; easing.bezierCurve: QsLook.unfold }
    }

    implicitHeight: root.shownHeight
    clip: true
    Accessible.role: Accessible.Grouping
    Accessible.name: I18n.tr("Now playing")

    Plate {
        anchors.fill: parent
        s: root.s
    }

    Row {
        anchors.left: parent.left
        anchors.leftMargin: Math.round(14 * root.s)
        anchors.verticalCenter: parent.verticalCenter
        spacing: Math.round(10 * root.s)
        opacity: root.playing ? 0 : 1
        visible: opacity > 0
        Behavior on opacity {
            NumberAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
        }

        Symbol {
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            name: "music_note"
            size: 18
            ink: Lithic.fg3
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("Nothing playing")
            color: Lithic.fg3
            font.family: QsLook.sans
            font.pixelSize: Math.round(13 * root.s)
            renderType: Text.NativeRendering
        }
    }

    Item {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: Math.round(72 * root.s)
        opacity: root.playing ? 1 : 0
        visible: opacity > 0
        Behavior on opacity {
            NumberAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
        }

        Item {
            id: art
            visible: root.showArtwork
            anchors.left: parent.left
            anchors.leftMargin: root.pad
            anchors.verticalCenter: parent.verticalCenter
            width: root.artSide
            height: root.artSide

            CutRect {
                visible: QsLook.lithic
                anchors.fill: parent
                cut: root.artCut
                fill: Lithic.field
            }
            CutRect {
                id: artMask
                anchors.fill: parent
                visible: false
                layer.enabled: QsLook.lithic
                cut: root.artCut
                fill: "black"
            }
            Image {
                id: cutCover
                visible: QsLook.lithic
                anchors.fill: parent
                source: QsLook.lithic ? root.coverSource : ""
                sourceSize.width: root.artSide * 2
                sourceSize.height: root.artSide * 2
                fillMode: Image.PreserveAspectCrop
                asynchronous: true
                cache: true
                opacity: status === Image.Ready ? 1 : 0
                layer.enabled: QsLook.lithic && status === Image.Ready
                layer.effect: MultiEffect {
                    maskEnabled: true
                    maskSource: artMask
                    maskThresholdMin: 0.5
                    maskSpreadAtMin: 1.0
                }
                Behavior on opacity {
                    NumberAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                }
            }
            CutRect {
                visible: QsLook.lithic
                anchors.fill: parent
                cut: root.artCut
                strokeColor: Lithic.vein
                strokeWidth: Math.max(1, Math.round(root.s))
            }
            ClippingRectangle {
                visible: !QsLook.lithic
                anchors.fill: parent
                radius: Look.cornerAt(LithicTokens.cut.small, width, height, root.s)
                color: QsLook.material ? Lithic.container : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.1 : 0.08)

                Image {
                    id: roundCover
                    anchors.fill: parent
                    source: QsLook.lithic ? "" : root.coverSource
                    sourceSize.width: root.artSide * 2
                    sourceSize.height: root.artSide * 2
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    opacity: status === Image.Ready ? 1 : 0
                    Behavior on opacity {
                        NumberAnimation { duration: QsLook.dur(QsLook.stateMs * 2) }
                    }
                }
            }
            Symbol {
                anchors.centerIn: parent
                visible: !root.artReady
                s: root.s
                name: "music_note"
                size: 22
                ink: QsLook.material ? Lithic.containerInk : Lithic.fg3
            }
        }

        Column {
            id: track
            visible: root.showTrack
            anchors.left: art.visible ? art.right : parent.left
            anchors.leftMargin: art.visible ? Math.round(12 * root.s) : Math.round(14 * root.s)
            anchors.right: transport.visible ? transport.left : parent.right
            anchors.rightMargin: Math.round(8 * root.s)
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round(2 * root.s)

            Text {
                width: parent.width
                text: root.title.length > 0 ? root.title : I18n.tr("Unknown track")
                color: Lithic.fg
                font.family: QsLook.sans
                font.pixelSize: Math.round(14 * root.s)
                font.weight: Font.Medium
                renderType: Text.NativeRendering
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                visible: root.artists.length > 0
                text: root.artists
                color: Lithic.fg2
                font.family: QsLook.sans
                font.pixelSize: Math.round(12 * root.s)
                renderType: Text.NativeRendering
                elide: Text.ElideRight
            }

            NumberAnimation {
                id: settle
                target: track
                property: "opacity"
                from: 0.3
                to: 1
                duration: QsLook.dur(QsLook.stateMs * 2)
                easing.type: Easing.OutCubic
            }
            Connections {
                target: root
                function onTitleChanged() {
                    if (root.active && root.title.length > 0)
                        settle.restart();
                }
            }
        }

        Row {
            id: transport
            visible: root.showTransport
            anchors.right: parent.right
            anchors.rightMargin: root.pad
            anchors.verticalCenter: parent.verticalCenter
            spacing: Math.round((QsLook.material ? 4 : 2) * root.s)

            CornerButton {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                glyph: "skip_previous"
                subtle: true
                enabled: root.player !== null && root.player.canGoPrevious
                Accessible.name: I18n.tr("Previous")
                onClicked: if (root.player) root.player.previous()
            }
            CornerButton {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                glyph: Media.playing ? "pause" : "play_arrow"
                emphasis: QsLook.material
                subtle: QsLook.fluent
                enabled: root.player !== null
                Accessible.name: Media.playing ? I18n.tr("Pause") : I18n.tr("Play")
                onClicked: Media.toggle()
            }
            CornerButton {
                anchors.verticalCenter: parent.verticalCenter
                s: root.s
                glyph: "skip_next"
                subtle: true
                enabled: root.player !== null && root.player.canGoNext
                Accessible.name: I18n.tr("Next")
                onClicked: if (root.player) root.player.next()
            }
        }
    }
}
